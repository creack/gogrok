package main

import (
	"context"
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
)

type listener struct {
	conn net.Conn
	ch   chan struct{}
}

func (l *listener) Accept() (net.Conn, error) {
	l.ch <- struct{}{}
	log.Printf("Accept!\n")
	return l.conn, nil
}

func (l *listener) Addr() net.Addr {
	return l.conn.LocalAddr()
}

func (l *listener) Close() error {
	close(l.ch)
	log.Printf("Attempt to close.")
	return nil
}

func run(ctx context.Context) error {
	var d net.Dialer

	conn, err := d.DialContext(ctx, "tcp", "localhost:9090")
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }() // Best effort.

	fmt.Fprintf(conn, "GET /new HTTP/1.1\r\nHost: localhost\r\n\r\n")

	l := &listener{
		conn: conn,
		ch:   make(chan struct{}, 1),
	}

	u, _ := url.Parse("http://localhost:" + os.Args[1])
	rp := httputil.NewSingleHostReverseProxy(u)

	srv := &http.Server{
		Handler: rp,
	}

	if err := srv.Serve(l); err != nil {
		return fmt.Errorf("serve: %w", err)
	}

	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		println("Fail:", err.Error())
		return
	}
	println("success")
}
