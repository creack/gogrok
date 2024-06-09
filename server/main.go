package main

import (
	"errors"
	"fmt"
	"io"
	"log"
	"net"
	"net/http"
	"strings"
	"sync"

	"github.com/google/uuid"
)

type controller struct {
	mu      sync.Mutex
	tunnels map[string]net.Conn
}

func (c *controller) handleCreateTunnel(w http.ResponseWriter, req *http.Request) {
	hj, ok := w.(http.Hijacker)
	if !ok {
		panic("w is not a hijacker") // Should never happen.
	}
	conn, _, err := hj.Hijack()
	if err != nil {
		log.Printf("Error hujacking: %s.", fmt.Errorf("hijack: %w", err))
		return
	}
	tunnelID := "3bb01403-9cb1-49c3-8b4a-f33593a833c6"
	_ = uuid.New().String()
	c.mu.Lock()
	c.tunnels[tunnelID] = conn
	c.mu.Unlock()
	log.Printf("%s\n", tunnelID)
}

type nopCloserConn struct {
	net.Conn
}

func (nopCloserConn) Close() error {
	log.Printf("Attempt to close!\n")
	return nil
}

// [*].grok.creack.net -> here.
func (c *controller) handleForward(w http.ResponseWriter, req *http.Request) {
	tunnelID := strings.Split(req.Host, ".")[0]
	c.mu.Lock()
	conn, ok := c.tunnels[tunnelID]
	c.mu.Unlock()
	if !ok || conn == nil {
		http.Error(w, "Target not found", http.StatusNotFound)
		return
	}

	client := &http.Client{
		Transport: &http.Transport{
			Dial: func(network, addr string) (net.Conn, error) {
				return &nopCloserConn{Conn: conn}, nil
			},
		},
	}

	log.Printf(">>> %s\n", req.URL.String())
	req2, _ := http.NewRequestWithContext(req.Context(), req.Method, "http://localhost"+req.URL.String(), req.Body)
	for k := range req.Header {
		req2.Header.Set(k, req.Header.Get(k))
	}

	resp, err := client.Do(req2)
	if err != nil {
		http.Error(w, fmt.Errorf("do forward: %w", err).Error(), http.StatusBadGateway)
		return
	}
	for k := range resp.Header {
		w.Header().Set(k, resp.Header.Get(k))
	}
	w.WriteHeader(resp.StatusCode)

	_, _ = io.Copy(w, resp.Body) // Best effort.
	_ = resp.Body.Close()        // Best effort.
}

func run() error {
	c := &controller{
		tunnels: map[string]net.Conn{},
	}

	go func() {
		forwardMux := http.NewServeMux()
		forwardMux.HandleFunc("/", c.handleForward)
		if err := http.ListenAndServeTLS(":9091", "/home/creack/certbot/grok.creack.net/cert1.pem", "/home/creack/certbot/grok.creack.net/privkey1.pem", forwardMux); err != nil && errors.Is(err, http.ErrServerClosed) {
			log.Printf("http.ListenAndServe 9091: %s.", fmt.Errorf("http.ListenAndServe: %w", err))
		}
	}()

	apiMux := http.NewServeMux()
	apiMux.HandleFunc("/new", c.handleCreateTunnel)
	if err := http.ListenAndServe(":9090", apiMux); err != nil && errors.Is(err, http.ErrServerClosed) {
		return fmt.Errorf("http.ListenAndServe: %w", err)
	}
	return nil
}

func main() {
	if err := run(); err != nil {
		println("Fail:", err.Error())
		return
	}
}
