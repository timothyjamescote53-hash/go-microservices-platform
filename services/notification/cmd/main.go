package main

import (
	"context"
	"encoding/json"
	"fmt"
	"log/slog"
	"net"
	"net/http"
	"os"
	"os/signal"
	"sync"
	"syscall"
	"time"
)

type NotificationType string

const (
	TypeEmail NotificationType = "email"
	TypeSMS   NotificationType = "sms"
	TypePush  NotificationType = "push"
)

type Notification struct {
	ID        string           `json:"id"`
	UserID    string           `json:"user_id"`
	Type      NotificationType `json:"type"`
	Subject   string           `json:"subject"`
	Body      string           `json:"body"`
	SentAt    *time.Time       `json:"sent_at,omitempty"`
	CreatedAt time.Time        `json:"created_at"`
}

type Store struct {
	mu            sync.RWMutex
	notifications map[string][]*Notification
}

func NewStore() *Store { return &Store{notifications: make(map[string][]*Notification)} }

func (s *Store) Save(n *Notification) {
	s.mu.Lock()
	s.notifications[n.UserID] = append(s.notifications[n.UserID], n)
	s.mu.Unlock()
}

func (s *Store) GetByUser(userID string) []*Notification {
	s.mu.RLock()
	defer s.mu.RUnlock()
	out := make([]*Notification, len(s.notifications[userID]))
	copy(out, s.notifications[userID])
	return out
}

type Service struct{ store *Store }

func (s *Service) Send(userID string, nType NotificationType, subject, body string) *Notification {
	now := time.Now()
	n := &Notification{
		ID: fmt.Sprintf("notif-%d", now.UnixNano()),
		UserID: userID, Type: nType,
		Subject: subject, Body: body,
		SentAt: &now, CreatedAt: now,
	}
	s.store.Save(n)
	slog.Info("Notification sent", "user", userID, "type", nType)
	return n
}

type handler struct{ svc *Service }

func (h *handler) send(w http.ResponseWriter, r *http.Request) {
	var req struct {
		UserID  string           `json:"user_id"`
		Type    NotificationType `json:"type"`
		Subject string           `json:"subject"`
		Body    string           `json:"body"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil || req.UserID == "" {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	n := h.svc.Send(req.UserID, req.Type, req.Subject, req.Body)
	writeJSON(w, http.StatusCreated, n)
}

func (h *handler) list(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	notifs := h.svc.store.GetByUser(userID)
	if notifs == nil { notifs = []*Notification{} }
	writeJSON(w, http.StatusOK, notifs)
}

func writeJSON(w http.ResponseWriter, status int, v any) {
	w.Header().Set("Content-Type", "application/json")
	w.WriteHeader(status)
	json.NewEncoder(w).Encode(v)
}

func getEnv(key, fallback string) string {
	if v := os.Getenv(key); v != "" { return v }
	return fallback
}

func main() {
	svc := &Service{store: NewStore()}
	h := &handler{svc: svc}
	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/notifications", h.send)
	mux.HandleFunc("GET /api/v1/notifications", h.list)
	mux.HandleFunc("GET /healthz/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	mux.HandleFunc("GET /healthz/ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})
	port := getEnv("HTTP_PORT", "8083")
	srv := &http.Server{Addr: net.JoinHostPort("", port), Handler: mux}
	go func() {
		slog.Info("Notification service started", "port", port)
		if err := srv.ListenAndServe(); err != nil && err != http.ErrServerClosed { os.Exit(1) }
	}()
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, syscall.SIGINT, syscall.SIGTERM)
	<-quit
	ctx, cancel := context.WithTimeout(context.Background(), 30*time.Second)
	defer cancel()
	srv.Shutdown(ctx)
}
// scaffold
// send
// list
// scaffold
// send
// list
// scaffold
// domain
// store
// send
// list
// log send
// method handler
// writeJSON
// getenv
// net join
// slog
// metrics endpoint
// context timeout
// signal notify
// scaffold
// domain
// store
// send
// list
// log send
// method handler
// writeJSON
// getenv
// net join
// slog
// metrics endpoint
// context timeout
// signal notify
// sent at auto
// type validate
// body required
// user required
// cors header
// version endpoint
// scaffold
// domain
// store
// send
// list
// log send
// method handler
// writeJSON
// getenv
// net join
// slog
// metrics endpoint
