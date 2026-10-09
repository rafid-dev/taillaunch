package proxy

import (
	"context"
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"
	"syscall"
	"time"
)

// DialContext is the dial function used by the proxy after routing policy has
// selected either the host network or the tailnet.
type DialContext func(ctx context.Context, network, address string) (net.Conn, error)

type Server struct {
	ListenAddr string
	Dial       DialContext
	Logf       func(format string, args ...any)

	ln        net.Listener
	http      *http.Server
	transport *http.Transport
	once      sync.Once
}

func (s *Server) Start() (string, error) {
	if s.Dial == nil {
		return "", errors.New("proxy: Dial is required")
	}
	addr := s.ListenAddr
	if addr == "" {
		addr = "127.0.0.1:0"
	}
	ln, err := net.Listen("tcp", addr)
	if err != nil {
		return "", err
	}
	if tcp, ok := ln.Addr().(*net.TCPAddr); !ok || !tcp.IP.IsLoopback() {
		_ = ln.Close()
		return "", fmt.Errorf("proxy: refusing non-loopback listener %q", ln.Addr())
	}

	s.transport = &http.Transport{
		Proxy:                 nil,
		DialContext:           s.Dial,
		ForceAttemptHTTP2:     false,
		MaxIdleConns:          32,
		IdleConnTimeout:       60 * time.Second,
		TLSHandshakeTimeout:   15 * time.Second,
		ExpectContinueTimeout: time.Second,
	}
	s.http = &http.Server{
		Handler:           s,
		ReadHeaderTimeout: 15 * time.Second,
		IdleTimeout:       90 * time.Second,
		ErrorLog:          log.New(io.Discard, "", 0),
	}
	s.ln = ln
	go func() {
		if err := s.http.Serve(ln); err != nil && !errors.Is(err, http.ErrServerClosed) {
			s.logf("proxy server stopped: %v", err)
		}
	}()
	return ln.Addr().String(), nil
}

func (s *Server) Close(ctx context.Context) error {
	var err error
	s.once.Do(func() {
		if s.transport != nil {
			s.transport.CloseIdleConnections()
		}
		if s.http != nil {
			err = s.http.Shutdown(ctx)
		}
	})
	return err
}

func (s *Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.targetsSelf(r) {
		http.Error(w, "TailLaunch proxy requires an absolute-URI request to another host", http.StatusBadRequest)
		return
	}
	if r.Method == http.MethodConnect {
		s.handleConnect(w, r)
		return
	}
	if isUpgrade(r) {
		s.handleUpgrade(w, r)
		return
	}
	s.handleHTTP(w, r)
}

func (s *Server) handleConnect(w http.ResponseWriter, r *http.Request) {
	address := r.Host
	if _, _, err := net.SplitHostPort(address); err != nil {
		address = net.JoinHostPort(strings.Trim(address, "[]"), "443")
	}

	upstream, err := s.Dial(r.Context(), "tcp", address)
	if err != nil {
		http.Error(w, "TailLaunch could not connect to destination: "+err.Error(), http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "proxy hijacking unsupported", http.StatusInternalServerError)
		return
	}
	client, rw, err := hj.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	defer client.Close()
	defer upstream.Close()

	if _, err := io.WriteString(client, "HTTP/1.1 200 Connection Established\r\n\r\n"); err != nil {
		return
	}
	if rw.Reader.Buffered() > 0 {
		if _, err := io.CopyN(upstream, rw, int64(rw.Reader.Buffered())); err != nil {
			return
		}
	}
	tunnel(client, upstream)
}

func (s *Server) handleHTTP(w http.ResponseWriter, r *http.Request) {
	out := r.Clone(r.Context())
	out.RequestURI = ""
	if out.URL.Scheme == "" {
		out.URL.Scheme = "http"
	}
	if out.URL.Host == "" {
		out.URL.Host = r.Host
	}
	removeHopHeaders(out.Header)

	resp, err := s.transport.RoundTrip(out)
	if err != nil {
		http.Error(w, "TailLaunch proxy error: "+err.Error(), http.StatusBadGateway)
		return
	}
	defer resp.Body.Close()
	copyHeader(w.Header(), resp.Header)
	removeHopHeaders(w.Header())
	w.WriteHeader(resp.StatusCode)
	copyAndFlush(w, resp.Body)
}

// copyAndFlush copies src to w, flushing after each chunk so streaming
// responses such as server-sent events reach the client as they arrive.
func copyAndFlush(w http.ResponseWriter, src io.Reader) {
	rc := http.NewResponseController(w)
	_ = rc.Flush()
	buf := make([]byte, 32*1024)
	for {
		n, err := src.Read(buf)
		if n > 0 {
			if _, werr := w.Write(buf[:n]); werr != nil {
				return
			}
			_ = rc.Flush()
		}
		if err != nil {
			return
		}
	}
}

// targetsSelf reports whether r cannot be proxied safely: a non-CONNECT
// request without an absolute URI, or any destination that is this proxy's
// own listener (which would make the proxy dial itself recursively).
func (s *Server) targetsSelf(r *http.Request) bool {
	host, defaultPort := r.Host, "443"
	if r.Method != http.MethodConnect {
		if r.URL.Host == "" {
			return true
		}
		host, defaultPort = r.URL.Host, "80"
		if r.URL.Scheme == "https" {
			defaultPort = "443"
		}
	}
	if s.ln == nil {
		return false
	}
	listen, ok := s.ln.Addr().(*net.TCPAddr)
	if !ok {
		return false
	}
	h, p, err := net.SplitHostPort(host)
	if err != nil {
		h, p = strings.Trim(host, "[]"), defaultPort
	}
	if p != fmt.Sprint(listen.Port) {
		return false
	}
	if strings.EqualFold(h, "localhost") {
		return true
	}
	ip := net.ParseIP(h)
	return ip != nil && ip.Equal(listen.IP)
}

func (s *Server) handleUpgrade(w http.ResponseWriter, r *http.Request) {
	address := r.URL.Host
	if address == "" {
		address = r.Host
	}
	if _, _, err := net.SplitHostPort(address); err != nil {
		address = net.JoinHostPort(strings.Trim(address, "[]"), "80")
	}

	upstream, err := s.Dial(r.Context(), "tcp", address)
	if err != nil {
		http.Error(w, "TailLaunch could not connect to destination: "+err.Error(), http.StatusBadGateway)
		return
	}

	hj, ok := w.(http.Hijacker)
	if !ok {
		_ = upstream.Close()
		http.Error(w, "proxy hijacking unsupported", http.StatusInternalServerError)
		return
	}
	client, rw, err := hj.Hijack()
	if err != nil {
		_ = upstream.Close()
		return
	}
	defer client.Close()
	defer upstream.Close()

	out := r.Clone(r.Context())
	out.RequestURI = ""
	if out.URL.Scheme == "" {
		out.URL.Scheme = "http"
	}
	if out.URL.Host == "" {
		out.URL.Host = r.Host
	}
	if err := out.Write(upstream); err != nil {
		return
	}
	if rw.Reader.Buffered() > 0 {
		_, _ = io.CopyN(upstream, rw, int64(rw.Reader.Buffered()))
	}
	tunnel(client, upstream)
}

func tunnel(a, b net.Conn) {
	var wg sync.WaitGroup
	wg.Add(2)
	go func() {
		defer wg.Done()
		_, _ = io.Copy(a, b)
		closeWrite(a)
	}()
	go func() {
		defer wg.Done()
		_, _ = io.Copy(b, a)
		closeWrite(b)
	}()
	wg.Wait()
}

func closeWrite(c net.Conn) {
	if cw, ok := c.(interface{ CloseWrite() error }); ok {
		_ = cw.CloseWrite()
	}
}

func isUpgrade(r *http.Request) bool {
	return strings.Contains(strings.ToLower(r.Header.Get("Connection")), "upgrade") && r.Header.Get("Upgrade") != ""
}

func removeHopHeaders(h http.Header) {
	// RFC 7230 allows the Connection header to nominate additional hop-by-hop
	// fields. Remove those first, then the standard hop-by-hop headers.
	for _, connection := range h.Values("Connection") {
		for _, token := range strings.Split(connection, ",") {
			if token = strings.TrimSpace(token); token != "" {
				h.Del(token)
			}
		}
	}
	for _, key := range []string{
		"Connection", "Proxy-Connection", "Keep-Alive", "Proxy-Authenticate",
		"Proxy-Authorization", "Te", "Trailer", "Transfer-Encoding", "Upgrade",
	} {
		h.Del(key)
	}
}

func copyHeader(dst, src http.Header) {
	for k, vv := range src {
		for _, v := range vv {
			dst.Add(k, v)
		}
	}
}

func (s *Server) logf(format string, args ...any) {
	if s.Logf != nil {
		s.Logf(format, args...)
	}
}

// ControlNotSelf is a net.Dialer.Control hook for the host dialer. It runs
// against the resolved address of each connection attempt and rejects any
// loopback or unspecified (0.0.0.0, ::) address on the proxy's own listener
// port, so a hostname that resolves to loopback cannot make the proxy dial
// itself. Linux and macOS route connections to unspecified addresses to
// loopback listeners.
func (s *Server) ControlNotSelf(network, address string, _ syscall.RawConn) error {
	if s.ln == nil {
		return nil
	}
	listen, ok := s.ln.Addr().(*net.TCPAddr)
	if !ok {
		return nil
	}
	h, p, err := net.SplitHostPort(address)
	if err != nil || p != fmt.Sprint(listen.Port) {
		return nil
	}
	if ip := net.ParseIP(h); ip != nil && (ip.IsLoopback() || ip.IsUnspecified()) {
		return errors.New("proxy: refusing to dial the proxy's own listener")
	}
	return nil
}
