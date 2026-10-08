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
