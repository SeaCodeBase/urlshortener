package model

import "time"

// Webhook status values (webhooks.status).
const (
	WebhookStatusActive      = "active"
	WebhookStatusAlert       = "alert"
	WebhookStatusDeactivated = "deactivated"
)

// Webhook receives signed engagement events for a group. Secret signs each
// body (HMAC-SHA256) and is only returned to the group's members.
// ConsecutiveFailures and FailingSince drive the alert/deactivated statuses.
type Webhook struct {
	ID                  uint64    `db:"id" json:"id"`
	UserID              uint64    `db:"user_id" json:"user_id"`
	GroupID             uint64    `db:"group_id" json:"group_id"`
	Name                string    `db:"name" json:"name"`
	URL                 string    `db:"url" json:"url"`
	Event               string    `db:"event" json:"event"`
	Secret              string    `db:"secret" json:"secret"`
	IsActive            bool      `db:"is_active" json:"is_active"`
	Status              string    `db:"status" json:"status"`
	ConsecutiveFailures int       `db:"consecutive_failures" json:"consecutive_failures"`
	FailingSince        NullTime  `db:"failing_since" json:"failing_since"`
	LastDeliveryAt      NullTime  `db:"last_delivery_at" json:"last_delivery_at"`
	CreatedAt           time.Time `db:"created_at" json:"created_at"`
	UpdatedAt           time.Time `db:"updated_at" json:"updated_at"`
}

// WebhookDelivery is one delivery attempt; StatusCode 0 means no HTTP
// response (Error says why).
type WebhookDelivery struct {
	ID          uint64    `db:"id" json:"id"`
	WebhookID   uint64    `db:"webhook_id" json:"webhook_id"`
	EventID     string    `db:"event_id" json:"event_id"`
	StatusCode  int       `db:"status_code" json:"status_code"`
	Attempt     int       `db:"attempt" json:"attempt"`
	DeliveredAt time.Time `db:"delivered_at" json:"delivered_at"`
	Error       string    `db:"error" json:"error"`
}
