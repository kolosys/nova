package main

import (
	"context"
	"fmt"
	"sync/atomic"

	"github.com/kolosys/nova/memory"
	"github.com/kolosys/nova/shared"
)

// UserEventListener handles user-related events
type UserEventListener struct {
	id           string
	eventsHandled int64
}

func (l *UserEventListener) ID() string {
	return l.id
}

func (l *UserEventListener) Handle(_ context.Context, event shared.Event) error {
	atomic.AddInt64(&l.eventsHandled, 1)

	switch event.Type() {
	case "user.created":
		return l.handleUserCreated(event)
	case "user.updated":
		return l.handleUserUpdated(event)
	default:
		fmt.Printf("👤 User listener received unknown event type: %s\n", event.Type())
	}

	return nil
}

func (l *UserEventListener) OnError(_ context.Context, event shared.Event, err error) error {
	fmt.Printf("❌ User listener error for event %s: %v\n", event.ID(), err)
	return err
}

func (l *UserEventListener) handleUserCreated(event shared.Event) error {
	data, ok := event.Data().(map[string]any)
	if !ok {
		return fmt.Errorf("invalid user data format")
	}
	
	name, _ := data["name"].(string)
	email, _ := data["email"].(string)
	userID, _ := event.Metadata()["user_id"]
	
	fmt.Printf("👤 User created: ID=%s, Name=%s, Email=%s\n", userID, name, email)
	
	// Simulate processing time
	// time.Sleep(10 * time.Millisecond)
	
	return nil
}

func (l *UserEventListener) handleUserUpdated(event shared.Event) error {
	data, ok := event.Data().(map[string]any)
	if !ok {
		return fmt.Errorf("invalid user data format")
	}
	
	name, _ := data["name"].(string)
	status, _ := data["status"].(string)
	userID, _ := event.Metadata()["user_id"]
	
	fmt.Printf("👤 User updated: ID=%s, Name=%s, Status=%s\n", userID, name, status)
	
	return nil
}

// OrderEventListener handles order-related events
type OrderEventListener struct {
	id           string
	eventsHandled int64
	totalRevenue float64
}

func (l *OrderEventListener) ID() string {
	return l.id
}

func (l *OrderEventListener) Handle(_ context.Context, event shared.Event) error {
	atomic.AddInt64(&l.eventsHandled, 1)

	switch event.Type() {
	case "order.created":
		return l.handleOrderCreated(event)
	case "order.updated":
		return l.handleOrderUpdated(event)
	case "order.cancelled":
		return l.handleOrderCancelled(event)
	default:
		fmt.Printf("🛒 Order listener received unknown event type: %s\n", event.Type())
	}

	return nil
}

func (l *OrderEventListener) OnError(_ context.Context, event shared.Event, err error) error {
	fmt.Printf("❌ Order listener error for event %s: %v\n", event.ID(), err)
	return err
}

func (l *OrderEventListener) handleOrderCreated(event shared.Event) error {
	data, ok := event.Data().(map[string]any)
	if !ok {
		return fmt.Errorf("invalid order data format")
	}
	
	amount, _ := data["amount"].(float64)
	customer, _ := data["customer"].(string)
	orderID, _ := event.Metadata()["order_id"]
	userID, _ := event.Metadata()["user_id"]
	
	l.totalRevenue += amount
	
	fmt.Printf("🛒 Order created: ID=%s, Customer=%s (User %s), Amount=$%.2f, Total Revenue: $%.2f\n", 
		orderID, customer, userID, amount, l.totalRevenue)
	
	return nil
}

func (l *OrderEventListener) handleOrderUpdated(event shared.Event) error {
	orderID, _ := event.Metadata()["order_id"]
	fmt.Printf("🛒 Order updated: ID=%s\n", orderID)
	return nil
}

func (l *OrderEventListener) handleOrderCancelled(event shared.Event) error {
	data, ok := event.Data().(map[string]any)
	if !ok {
		return fmt.Errorf("invalid order data format")
	}
	
	amount, _ := data["amount"].(float64)
	orderID, _ := event.Metadata()["order_id"]
	
	l.totalRevenue -= amount
	
	fmt.Printf("🛒 Order cancelled: ID=%s, Amount=$%.2f, Total Revenue: $%.2f\n", 
		orderID, amount, l.totalRevenue)
	
	return nil
}

// AuditEventListener logs all events for audit purposes
type AuditEventListener struct {
	id           string
	eventsHandled int64
	store        memory.EventStore
}

func (l *AuditEventListener) ID() string {
	return l.id
}

func (l *AuditEventListener) Handle(ctx context.Context, event shared.Event) error {
	atomic.AddInt64(&l.eventsHandled, 1)

	// Store in audit log
	auditEvent := shared.NewBaseEventWithMetadata(
		fmt.Sprintf("audit-%s", event.ID()),
		"audit.logged",
		map[string]any{
			"original_event_id":   event.ID(),
			"original_event_type": event.Type(),
			"original_timestamp":  event.Timestamp(),
			"original_data":       event.Data(),
		},
		map[string]string{
			"audit_timestamp": event.Timestamp().Format("2006-01-02T15:04:05Z07:00"),
			"audit_source":    "audit-listener",
		},
	)

	if err := l.store.Append(ctx, "audit-trail", auditEvent); err != nil {
		fmt.Printf("❌ Failed to store audit event: %v\n", err)
		return err
	}

	// Occasionally print audit summary (every 10th event)
	if l.eventsHandled%10 == 0 {
		fmt.Printf("📝 Audit: Logged %d events total\n", l.eventsHandled)
	}

	return nil
}

func (l *AuditEventListener) OnError(ctx context.Context, event shared.Event, err error) error {
	fmt.Printf("❌ Audit listener error for event %s: %v\n", event.ID(), err)

	// Try to log the error itself
	errorEvent := shared.NewBaseEventWithMetadata(
		fmt.Sprintf("audit-error-%s", event.ID()),
		"audit.error",
		map[string]any{
			"original_event_id": event.ID(),
			"error_message":     err.Error(),
		},
		map[string]string{
			"audit_timestamp": event.Timestamp().Format("2006-01-02T15:04:05Z07:00"),
			"audit_source":    "audit-listener-error",
		},
	)

	_ = l.store.Append(ctx, "audit-errors", errorEvent)

	return err
}
