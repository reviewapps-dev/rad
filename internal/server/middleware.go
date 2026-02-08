package server

import (
	"bufio"
	"fmt"
	"log"
	"net"
	"net/http"
	"strings"
	"time"
)

func (s *Server) authMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		header := r.Header.Get("Authorization")
		if header == "" {
			writeError(w, http.StatusUnauthorized, "missing authorization header")
			return
		}

		token := strings.TrimPrefix(header, "Bearer ")
		if token == header {
			writeError(w, http.StatusUnauthorized, "invalid authorization format")
			return
		}

		if token != s.cfg.Auth.Token {
			writeError(w, http.StatusUnauthorized, "invalid token")
			return
		}

		next.ServeHTTP(w, r)
	})
}

// streamAuthMiddleware accepts either the main Bearer token (via header) or
// the read-only stream token (via ?token= query param). Browser WebSocket APIs
// cannot set custom headers, so the query param is the primary auth mechanism
// for streaming clients.
func (s *Server) streamAuthMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Try Bearer header first (curl / API usage)
		if header := r.Header.Get("Authorization"); header != "" {
			token := strings.TrimPrefix(header, "Bearer ")
			if token != header && token == s.cfg.Auth.Token {
				next.ServeHTTP(w, r)
				return
			}
		}

		// Try ?token= query param (browser WebSocket)
		if qToken := r.URL.Query().Get("token"); qToken != "" {
			if qToken == s.cfg.Auth.Token {
				next.ServeHTTP(w, r)
				return
			}
			if s.cfg.Auth.StreamToken != "" && qToken == s.cfg.Auth.StreamToken {
				next.ServeHTTP(w, r)
				return
			}
		}

		writeError(w, http.StatusUnauthorized, "invalid or missing token")
	})
}

func loggingMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		start := time.Now()
		sw := &statusWriter{ResponseWriter: w, status: 200}
		next.ServeHTTP(sw, r)
		log.Printf("%s %s %d %s", r.Method, r.URL.Path, sw.status, time.Since(start).Round(time.Millisecond))
	})
}

func recoveryMiddleware(next http.Handler) http.Handler {
	return http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		defer func() {
			if err := recover(); err != nil {
				log.Printf("panic: %v", err)
				writeError(w, http.StatusInternalServerError, "internal server error")
			}
		}()
		next.ServeHTTP(w, r)
	})
}

type statusWriter struct {
	http.ResponseWriter
	status int
}

func (w *statusWriter) WriteHeader(status int) {
	w.status = status
	w.ResponseWriter.WriteHeader(status)
}

// Hijack forwards to the underlying ResponseWriter so WebSocket upgrades work.
func (w *statusWriter) Hijack() (net.Conn, *bufio.ReadWriter, error) {
	if h, ok := w.ResponseWriter.(http.Hijacker); ok {
		return h.Hijack()
	}
	return nil, nil, fmt.Errorf("underlying ResponseWriter does not support hijacking")
}
