package main

import "testing"

func newTestSvc() *OrderService { return &OrderService{store: NewStore()} }

func TestCreateOrder_Success(t *testing.T) {
	svc := newTestSvc()
	items := []OrderItem{{ProductID: "p1", Name: "Widget", Quantity: 2, UnitPrice: 9.99}}
	o, err := svc.CreateOrder("user-1", items)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if o.ID == "" { t.Error("expected order ID") }
	if o.TotalPrice != 19.98 { t.Errorf("expected 19.98, got %.2f", o.TotalPrice) }
	// Payment is synchronous so order should be COMPLETED
	got, _ := svc.GetOrder(o.ID)
	if got.Status != StatusCompleted { t.Errorf("expected COMPLETED, got %s", got.Status) }
}

func TestCreateOrder_EmptyItems(t *testing.T) {
	svc := newTestSvc()
	_, err := svc.CreateOrder("user-1", []OrderItem{})
	if err != ErrInvalidOrderData { t.Fatalf("expected ErrInvalidOrderData, got %v", err) }
}

func TestCreateOrder_MultipleItems(t *testing.T) {
	svc := newTestSvc()
	items := []OrderItem{
		{ProductID: "p1", Quantity: 1, UnitPrice: 10.00},
		{ProductID: "p2", Quantity: 3, UnitPrice: 5.00},
	}
	o, err := svc.CreateOrder("user-1", items)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if o.TotalPrice != 25.00 { t.Errorf("expected 25.00, got %.2f", o.TotalPrice) }
}

func TestGetOrder_Found(t *testing.T) {
	svc := newTestSvc()
	items := []OrderItem{{ProductID: "p1", Quantity: 1, UnitPrice: 10.00}}
	o, _ := svc.CreateOrder("user-1", items)
	got, err := svc.GetOrder(o.ID)
	if err != nil { t.Fatalf("unexpected error: %v", err) }
	if got.ID != o.ID { t.Error("ID mismatch") }
}

func TestGetOrder_NotFound(t *testing.T) {
	svc := newTestSvc()
	_, err := svc.GetOrder("bad-id")
	if err != ErrOrderNotFound { t.Fatalf("expected ErrOrderNotFound, got %v", err) }
}

func TestGetUserOrders(t *testing.T) {
	svc := newTestSvc()
	items := []OrderItem{{ProductID: "p1", Quantity: 1, UnitPrice: 1.00}}
	svc.CreateOrder("user-1", items)
	svc.CreateOrder("user-1", items)
	svc.CreateOrder("user-2", items)
	orders := svc.GetUserOrders("user-1")
	if len(orders) != 2 { t.Errorf("expected 2 orders for user-1, got %d", len(orders)) }
}

func TestOrderStatus_Transitions(t *testing.T) {
	store := NewStore()
	o := &Order{ID: "o1", Status: StatusPending}
	store.Create(o)
	store.UpdateStatus("o1", StatusProcessing)
	got, _ := store.GetByID("o1")
	if got.Status != StatusProcessing { t.Error("expected PROCESSING") }
}
// tests
// race test
// fix assertion
// assertion
// create ok
// empty items
// total
// get found
// not found
// user orders
// status
// race fix
// completed
// sync fix
// multi item
// empty user
// multi user
// zero price
// large order
// create ok
// empty items
// total
// get found
// not found
// user orders
// status
// race fix
// completed
// sync fix
// multi item
// empty user
// multi user
// zero price
// large order
// negative price
// zero qty
// user required
// concurrent create
// version test
