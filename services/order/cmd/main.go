package main

import (
	"context"
	"crypto/rand"
	"encoding/json"
	"errors"
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

func newUUID() string {
	b := make([]byte, 16)
	rand.Read(b)
	return fmt.Sprintf("%x-%x-%x-%x-%x", b[0:4], b[4:6], b[6:8], b[8:10], b[10:])
}

type OrderStatus string

const (
	StatusPending    OrderStatus = "PENDING"
	StatusProcessing OrderStatus = "PROCESSING"
	StatusCompleted  OrderStatus = "COMPLETED"
	StatusFailed     OrderStatus = "FAILED"
)

type OrderItem struct {
	ProductID string  `json:"product_id"`
	Name      string  `json:"name"`
	Quantity  int     `json:"quantity"`
	UnitPrice float64 `json:"unit_price"`
}

type Order struct {
	ID         string      `json:"id"`
	UserID     string      `json:"user_id"`
	Items      []OrderItem `json:"items"`
	TotalPrice float64     `json:"total_price"`
	Status     OrderStatus `json:"status"`
	CreatedAt  time.Time   `json:"created_at"`
	UpdatedAt  time.Time   `json:"updated_at"`
}

var (
	ErrOrderNotFound    = errors.New("order not found")
	ErrInvalidOrderData = errors.New("invalid order data")
)

type Store struct {
	mu     sync.RWMutex
	orders map[string]*Order
}

func NewStore() *Store { return &Store{orders: make(map[string]*Order)} }

func (s *Store) Create(o *Order) {
	s.mu.Lock()
	s.orders[o.ID] = o
	s.mu.Unlock()
}

func (s *Store) GetByID(id string) (*Order, error) {
	s.mu.RLock()
	defer s.mu.RUnlock()
	if o, ok := s.orders[id]; ok {
		cp := *o
		return &cp, nil
	}
	return nil, ErrOrderNotFound
}

func (s *Store) GetByUserID(userID string) []*Order {
	s.mu.RLock()
	defer s.mu.RUnlock()
	var out []*Order
	for _, o := range s.orders {
		if o.UserID == userID {
			cp := *o
			out = append(out, &cp)
		}
	}
	return out
}

func (s *Store) UpdateStatus(id string, status OrderStatus) {
	s.mu.Lock()
	if o, ok := s.orders[id]; ok {
		o.Status = status
		o.UpdatedAt = time.Now()
	}
	s.mu.Unlock()
}

type OrderService struct{ store *Store }

func (s *OrderService) CreateOrder(userID string, items []OrderItem) (*Order, error) {
	if len(items) == 0 {
		return nil, ErrInvalidOrderData
	}
	var total float64
	for _, item := range items {
		total += float64(item.Quantity) * item.UnitPrice
	}
	o := &Order{
		ID: newUUID(), UserID: userID, Items: items,
		TotalPrice: total, Status: StatusPending,
		CreatedAt: time.Now(), UpdatedAt: time.Now(),
	}
	s.store.Create(o)
	// Process payment synchronously to avoid data races in tests
	s.processPayment(o.ID)
	return o, nil
}

func (s *OrderService) processPayment(orderID string) {
	s.store.UpdateStatus(orderID, StatusProcessing)
	s.store.UpdateStatus(orderID, StatusCompleted)
}

func (s *OrderService) GetOrder(id string) (*Order, error)   { return s.store.GetByID(id) }
func (s *OrderService) GetUserOrders(userID string) []*Order { return s.store.GetByUserID(userID) }

type handler struct{ svc *OrderService }

func (h *handler) createOrder(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	var req struct {
		Items []OrderItem `json:"items"`
	}
	if err := json.NewDecoder(r.Body).Decode(&req); err != nil {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "invalid request"})
		return
	}
	o, err := h.svc.CreateOrder(userID, req.Items)
	if errors.Is(err, ErrInvalidOrderData) {
		writeJSON(w, http.StatusBadRequest, map[string]string{"error": "items required"})
		return
	}
	writeJSON(w, http.StatusCreated, o)
}

func (h *handler) getOrder(w http.ResponseWriter, r *http.Request) {
	id := r.PathValue("id")
	o, err := h.svc.GetOrder(id)
	if errors.Is(err, ErrOrderNotFound) {
		writeJSON(w, http.StatusNotFound, map[string]string{"error": "not found"})
		return
	}
	writeJSON(w, http.StatusOK, o)
}

func (h *handler) listOrders(w http.ResponseWriter, r *http.Request) {
	userID := r.Header.Get("X-User-ID")
	orders := h.svc.GetUserOrders(userID)
	if orders == nil {
		orders = []*Order{}
	}
	writeJSON(w, http.StatusOK, orders)
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
	svc := &OrderService{store: NewStore()}
	h := &handler{svc: svc}

	mux := http.NewServeMux()
	mux.HandleFunc("POST /api/v1/orders", h.createOrder)
	mux.HandleFunc("GET /api/v1/orders", h.listOrders)
	mux.HandleFunc("GET /api/v1/orders/{id}", h.getOrder)
	mux.HandleFunc("GET /healthz/live", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "alive"})
	})
	mux.HandleFunc("GET /healthz/ready", func(w http.ResponseWriter, r *http.Request) {
		writeJSON(w, http.StatusOK, map[string]string{"status": "ready"})
	})

	port := getEnv("HTTP_PORT", "8082")
	srv := &http.Server{Addr: net.JoinHostPort("", port), Handler: mux}
	go func() {
		slog.Info("Order service started", "port", port)
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
}
// scaffold
// order domain
// order store
// create order
// payment
// get order
// list orders
// fix race
// race fix
// scaffold
// domain
// status
// store
// create
// payment
// status update
// get order
// list orders
// create handler
// health
// log create
// log payment
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
// status
// store
// create
// payment
// status update
// get order
// list orders
// create handler
// health
// log create
// log payment
// method handler
// writeJSON
// getenv
// net join
// slog
// metrics endpoint
