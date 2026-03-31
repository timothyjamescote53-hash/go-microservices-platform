package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"net/http/httputil"
	"net/url"
	"os"
	"os/signal"
	"strings"
	"sync"
	"syscall"
	"time"
)

// ── Rate limiter (token bucket, per IP) ──────────────────────────────────────

type bucket struct {
	tokens   float64
	lastSeen time.Time
}

type RateLimiter struct {
	mu      sync.Mutex
	buckets map[string]*bucket
	rate    float64
	burst   float64
}

func NewRateLimiter(rps, burst float64) *RateLimiter {
	rl := &RateLimiter{buckets: make(map[string]*bucket), rate: rps, burst: burst}
	go rl.cleanup()
	return rl
}

func (rl *RateLimiter) Allow(ip string) bool {
	rl.mu.Lock()
	defer rl.mu.Unlock()
	b, ok := rl.buckets[ip]
	if !ok {
		b = &bucket{tokens: rl.burst, lastSeen: time.Now()}
		rl.buckets[ip] = b
	}
	elapsed := time.Since(b.lastSeen).Seconds()
	b.tokens = min(rl.burst, b.tokens+elapsed*rl.rate)
	b.lastSeen = time.Now()
	if b.tokens < 1 {
		return false
	}
	b.tokens--
	return true
}

func (rl *RateLimiter) cleanup() {
	for range time.NewTicker(time.Minute).C {
		rl.mu.Lock()
		for ip, b := range rl.buckets {
			if time.Since(b.lastSeen) > 5*time.Minute {
				delete(rl.buckets, ip)
			}
		}
		rl.mu.Unlock()
	}
}

func min(a, b float64) float64 {
	if a < b {
		return a
	}
	return b
}

// ── Gateway ───────────────────────────────────────────────────────────────────

type Gateway struct {
	limiter *RateLimiter
	routes  map[string]string // prefix → upstream URL
}

func NewGateway(routes map[string]string) *Gateway {
	return &Gateway{limiter: NewRateLimiter(100, 200), routes: routes}
}

func (g *Gateway) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	// Rate limit
	ip := r.RemoteAddr
	if i := strings.LastIndex(ip, ":"); i > 0 {
		ip = ip[:i]
	}
	if !g.limiter.Allow(ip) {
		writeJSON(w, http.StatusTooManyRequests, map[string]string{"error": "rate limit exceeded"})
		return
	}

	// Health and metrics passthrough
	if r.URL.Path == "/healthz/live" || r.URL.Path == "/healthz/ready" {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
		return
	}
	if r.URL.Path == "/metrics" {
		fmt.Fprintln(w, "# api-gateway metrics")
		return
	}

	// Route to upstream
	for prefix, upstream := range g.routes {
		if strings.HasPrefix(r.URL.Path, prefix) {
			target, _ := url.Parse(upstream)
			proxy := httputil.NewSingleHostReverseProxy(target)
			proxy.ErrorHandler = func(w http.ResponseWriter, r *http.Request, err error) {
				writeJSON(w, http.StatusBadGateway, map[string]string{"error": "upstream unavailable"})
			}
			r.Header.Set("X-Forwarded-For", ip)
			proxy.ServeHTTP(w, r)
			return
		}
	}

	writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" {
		return v
	}
	return fallback
}

func main() {
	routes := map[string]string{
		"/api/v1/auth":          getEnv("AUTH_SERVICE_URL", "http://auth-service:8080"),
		"/api/v1/users":         getEnv("USER_SERVICE_URL", "http://user-service:8081"),
		"/api/v1/orders":        getEnv("ORDER_SERVICE_URL", "http://order-service:8082"),
		"/api/v1/notifications": getEnv("NOTIFICATION_SERVICE_URL", "http://notification-service:8083"),
	}

	gw := NewGateway(routes)
	port := getEnv("HTTP_PORT", "8000")
	srv := &http.Server{
		Addr:         net.JoinHostPort("", port),
		Handler:      gw,
		ReadTimeout:  15 * time.Second,
		WriteTimeout: 15 * time.Second,
	}

	go func() {
		slog.Info("API Gateway started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed {
			os.Exit(1)
		}
	}()

	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
	slog.Info("API Gateway stopped")
}
// scaffold
// rate limiter
// reverse proxy
// health
// error handler
// scaffold
// rate limiter
// refill fix
// proxy
// health
// error
// cleanup
// scaffold
// rate limiter
// bucket refill
// burst fix
// cleanup
// proxy
// routes
// error handler
// health
// metrics
// server
// log rate
// log proxy
// writeJSON
// getenv
// net join
// slog
// context timeout
// signal notify
// scaffold
// rate limiter
// bucket refill
// burst fix
// cleanup
// proxy
// routes
// error handler
// health
// metrics
// server
// log rate
// log proxy
// writeJSON
// getenv
// net join
// slog
// context timeout
// signal notify
// x forward
// request id
// timeout
// cors
// version
// scaffold
// rate limiter
// bucket refill
// burst fix
// cleanup
// proxy
// routes
// error handler
// health
// metrics
// server
// log rate
// log proxy
// writeJSON
