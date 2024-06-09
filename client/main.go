package main

import (
	"context"
	"fmt"
	"io"
	"log"
	"net"
	"sync"
)

func run(ctx context.Context) error {
	conn, err := net.Dial("tcp", "localhost:9090")
	if err != nil {
		return fmt.Errorf("dial: %w", err)
	}
	defer func() { _ = conn.Close() }() // Best effort.

	fmt.Fprintf(conn, "GET /new HTTP/1.1\r\nHost: localhost\r\n\r\n")

	var mu sync.Mutex
	var conn2 net.Conn
	go func() {
		buf := make([]byte, 32*1024)
	loop:
		n, err := conn.Read(buf)
		if err != nil {
			log.Printf("read: %s", err)
			return
		}
		mu.Lock()
		conn22 := conn2
		mu.Unlock()
		if _, err := conn22.Write(buf[:n]); err != nil {
			log.Printf("write: %s", err)
			return
		}
		goto loop
	}()

loop:
	conn22, err := net.Dial("tcp", "localhost:8089")
	if err != nil {
		return fmt.Errorf("net.Dial2: %w", err)
	}
	mu.Lock()
	conn2 = conn22
	mu.Unlock()
	defer func() { _ = conn2.Close() }() // Best effort.

	go func() {
		defer log.Printf("conn2 << conn done")
		_, _ = io.Copy(conn2, conn)
	}()

	_, _ = io.Copy(conn, conn2)
	log.Printf("conn << conn2 done")
	goto loop
	return nil
}

func main() {
	if err := run(context.Background()); err != nil {
		println("Fail:", err.Error())
		return
	}
	println("success")
}
