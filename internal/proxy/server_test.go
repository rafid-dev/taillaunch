package proxy

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"net"
	"net/http"
	"net/http/httptest"
	"net/url"
	"strings"
	"sync/atomic"
	"testing"
	"time"
)

func TestHTTPProxy(t *testing.T) {
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprint(w, "ok:"+r.URL.Path)
	}))
	defer origin.Close()

	var dials atomic.Int32
	s := &Server{Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		dials.Add(1)
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	}}
	addr, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())

	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	resp, err := client.Get(origin.URL + "/hello")
	if err != nil {
		t.Fatal(err)
	}
	defer resp.Body.Close()
	body, _ := io.ReadAll(resp.Body)
	if got, want := string(body), "ok:/hello"; got != want {
		t.Fatalf("body=%q want=%q", got, want)
	}
	if dials.Load() == 0 {
		t.Fatal("expected proxy dial")
	}
}

func TestConnectTunnel(t *testing.T) {
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	defer ln.Close()
	go func() {
		for {
			c, err := ln.Accept()
			if err != nil {
				return
			}
			go func(c net.Conn) {
				defer c.Close()
				_, _ = io.Copy(c, c)
			}(c)
		}
	}()

	s := &Server{Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	}}
	proxyAddr, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())

	c, err := net.DialTimeout("tcp", proxyAddr, 2*time.Second)
	if err != nil {
		t.Fatal(err)
	}
	defer c.Close()
	fmt.Fprintf(c, "CONNECT %s HTTP/1.1\r\nHost: %s\r\n\r\n", ln.Addr(), ln.Addr())
	br := bufio.NewReader(c)
	line, err := br.ReadString('\n')
	if err != nil || !strings.Contains(line, "200") {
		t.Fatalf("CONNECT response: %q err=%v", line, err)
	}
	for {
		line, _ = br.ReadString('\n')
		if line == "\r\n" || line == "" {
			break
		}
	}
	if _, err := c.Write([]byte("ping")); err != nil {
		t.Fatal(err)
	}
	buf := make([]byte, 4)
	if _, err := io.ReadFull(br, buf); err != nil {
		t.Fatal(err)
	}
	if string(buf) != "ping" {
		t.Fatalf("got %q", buf)
	}
}

func TestRemoveHopHeadersRemovesConnectionTokens(t *testing.T) {
	h := http.Header{
		"Connection":       []string{"keep-alive, X-Hop-Only"},
		"X-Hop-Only":       []string{"secret"},
		"Proxy-Connection": []string{"keep-alive"},
		"X-End-To-End":     []string{"keep"},
	}
	removeHopHeaders(h)
	if got := h.Get("X-Hop-Only"); got != "" {
		t.Fatalf("X-Hop-Only survived: %q", got)
	}
	if got := h.Get("Connection"); got != "" {
		t.Fatalf("Connection survived: %q", got)
	}
	if got := h.Get("Proxy-Connection"); got != "" {
		t.Fatalf("Proxy-Connection survived: %q", got)
	}
	if got, want := h.Get("X-End-To-End"), "keep"; got != want {
		t.Fatalf("X-End-To-End=%q want=%q", got, want)
	}
}

func TestRejectsSelfAndOriginFormRequests(t *testing.T) {
	var dials atomic.Int32
	s := &Server{Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		// Cap dials so a regression fails the test instead of recursing until
		// the process runs out of file descriptors.
		if dials.Add(1) > 5 {
			return nil, fmt.Errorf("dial cap exceeded")
		}
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	}}
	addr, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	_, port, _ := net.SplitHostPort(addr)

	for _, tc := range []struct{ name, request string }{
		{"origin-form to proxy address", "GET / HTTP/1.1\r\nHost: " + addr + "\r\nConnection: close\r\n\r\n"},
		{"origin-form to other host", "GET / HTTP/1.1\r\nHost: example.invalid\r\nConnection: close\r\n\r\n"},
		{"absolute-form to proxy address", "GET http://" + addr + "/ HTTP/1.1\r\nHost: " + addr + "\r\nConnection: close\r\n\r\n"},
		{"absolute-form to localhost proxy port", "GET http://localhost:" + port + "/ HTTP/1.1\r\nHost: localhost:" + port + "\r\nConnection: close\r\n\r\n"},
		{"CONNECT to proxy address", "CONNECT " + addr + " HTTP/1.1\r\nHost: " + addr + "\r\nConnection: close\r\n\r\n"},
	} {
		t.Run(tc.name, func(t *testing.T) {
			dials.Store(0)
			c, err := net.DialTimeout("tcp", addr, 2*time.Second)
			if err != nil {
				t.Fatal(err)
			}
			defer c.Close()
			_ = c.SetDeadline(time.Now().Add(5 * time.Second))
			if _, err := io.WriteString(c, tc.request); err != nil {
				t.Fatal(err)
			}
			resp, err := http.ReadResponse(bufio.NewReader(c), nil)
			if err != nil {
				t.Fatal(err)
			}
			defer resp.Body.Close()
			if resp.StatusCode != http.StatusBadRequest {
				t.Fatalf("status=%d want=%d", resp.StatusCode, http.StatusBadRequest)
			}
			if n := dials.Load(); n != 0 {
				t.Fatalf("proxy dialed %d times, want 0", n)
			}
		})
	}
}

func TestHTTPProxyFlushesStreamingResponse(t *testing.T) {
	release := make(chan struct{})
	origin := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		w.Header().Set("Content-Type", "text/event-stream")
		fmt.Fprint(w, "data: first\n\n")
		w.(http.Flusher).Flush()
		select {
		case <-release:
		case <-r.Context().Done():
		}
		fmt.Fprint(w, "data: second\n\n")
	}))
	defer origin.Close()

	s := &Server{Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		var d net.Dialer
		return d.DialContext(ctx, network, address)
	}}
	addr, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	// Release the origin before Close so Shutdown is not left waiting on the open stream.
	defer close(release)

	proxyURL, _ := url.Parse("http://" + addr)
	client := &http.Client{Transport: &http.Transport{Proxy: http.ProxyURL(proxyURL)}}
	// The deadline bounds both the headers and the first body read, so a
	// buffering proxy fails the test instead of hanging it.
	ctx, cancel := context.WithTimeout(context.Background(), 2*time.Second)
	defer cancel()
	req, _ := http.NewRequestWithContext(ctx, http.MethodGet, origin.URL, nil)
	resp, err := client.Do(req)
	if err != nil {
		t.Fatalf("no response while origin stream still open (proxy is buffering): %v", err)
	}
	defer resp.Body.Close()
	line, err := bufio.NewReader(resp.Body).ReadString('\n')
	if err != nil {
		t.Fatalf("first event not delivered while origin stream still open (proxy is buffering): %v", err)
	}
	if line != "data: first\n" {
		t.Fatalf("first line=%q", line)
	}
}

// fakeDNS serves an A record of 127.0.0.1 for every name and empty answers for
// other record types, so the test has a hostname that resolves to loopback
// without touching the network.
func fakeDNS(t *testing.T) *net.Resolver {
	t.Helper()
	pc, err := net.ListenPacket("udp", "127.0.0.1:0")
	if err != nil {
		t.Fatal(err)
	}
	t.Cleanup(func() { pc.Close() })
	go func() {
		buf := make([]byte, 512)
		for {
			n, from, err := pc.ReadFrom(buf)
			if err != nil {
				return
			}
			if n < 12 {
				continue
			}
			q := buf[:n]
			end := 12
			for end < n && q[end] != 0 {
				end += int(q[end]) + 1
			}
			end += 5 // root label + QTYPE + QCLASS
			if end > n {
				continue
			}
			resp := append([]byte(nil), q[:end]...)
			resp[2], resp[3] = 0x81, 0x80 // response, recursion available, no error
			resp[6], resp[7], resp[8], resp[9], resp[10], resp[11] = 0, 0, 0, 0, 0, 0
			if qtype := uint16(q[end-4])<<8 | uint16(q[end-3]); qtype == 1 {
				resp[7] = 1
				resp = append(resp, 0xc0, 0x0c, 0, 1, 0, 1, 0, 0, 0, 60, 0, 4, 127, 0, 0, 1)
			}
			_, _ = pc.WriteTo(resp, from)
		}
	}()
	return &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			var d net.Dialer
			return d.DialContext(ctx, network, pc.LocalAddr().String())
		},
	}
}

func TestDialGuardRejectsHostnameResolvingToProxyListener(t *testing.T) {
	var dials atomic.Int32
	var s *Server
	resolver := fakeDNS(t)
	s = &Server{Dial: func(ctx context.Context, network, address string) (net.Conn, error) {
		// Cap dials so a regression fails the test instead of recursing until
		// the process runs out of file descriptors.
		if dials.Add(1) > 5 {
			return nil, fmt.Errorf("dial cap exceeded")
		}
		d := net.Dialer{Resolver: resolver, Control: s.ControlNotSelf}
		return d.DialContext(ctx, network, address)
	}}
	addr, err := s.Start()
	if err != nil {
		t.Fatal(err)
	}
	defer s.Close(context.Background())
	_, port, _ := net.SplitHostPort(addr)

	// Direct checks of the hook, independent of how the OS handles dialing a
	// wildcard address (Windows rewrites it to loopback before Control runs).
	for _, host := range []string{"0.0.0.0", "::", "127.0.0.2", "::1"} {
		if err := s.ControlNotSelf("tcp", net.JoinHostPort(host, port), nil); err == nil {
			t.Errorf("ControlNotSelf allowed %s on the listener port", host)
		}
	}
	if err := s.ControlNotSelf("tcp", net.JoinHostPort("0.0.0.0", "1"), nil); err != nil {
		t.Errorf("ControlNotSelf rejected another port: %v", err)
	}

	for _, host := range []string{"loopback-alias.test", "0.0.0.0", "[::]"} {
		hostport := host + ":" + port
		for _, tc := range []struct{ name, request string }{
			{"absolute-form", "GET http://" + hostport + "/ HTTP/1.1\r\nHost: " + hostport + "\r\nConnection: close\r\n\r\n"},
			{"CONNECT", "CONNECT " + hostport + " HTTP/1.1\r\nHost: " + hostport + "\r\nConnection: close\r\n\r\n"},
		} {
			t.Run(host+"/"+tc.name, func(t *testing.T) {
				dials.Store(0)
				c, err := net.DialTimeout("tcp", addr, 2*time.Second)
				if err != nil {
					t.Fatal(err)
				}
				defer c.Close()
				_ = c.SetDeadline(time.Now().Add(5 * time.Second))
				if _, err := io.WriteString(c, tc.request); err != nil {
					t.Fatal(err)
				}
				resp, err := http.ReadResponse(bufio.NewReader(c), nil)
				if err != nil {
					t.Fatal(err)
				}
				defer resp.Body.Close()
				body, _ := io.ReadAll(resp.Body)
				if resp.StatusCode != http.StatusBadGateway {
					t.Fatalf("status=%d want=%d", resp.StatusCode, http.StatusBadGateway)
				}
				if !strings.Contains(string(body), "refusing to dial the proxy's own listener") {
					t.Fatalf("body=%q does not report the dial guard", body)
				}
				if n := dials.Load(); n != 1 {
					t.Fatalf("proxy dialed %d times, want 1 (the guarded dial)", n)
				}
			})
		}
	}
}
