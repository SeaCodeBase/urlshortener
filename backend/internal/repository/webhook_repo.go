package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrWebhookNotFound = errors.New("webhook not found")

// webhookDeliveriesKept is how many deliveries AddDelivery keeps per webhook.
const webhookDeliveriesKept = 50

// Compile-time check: WebhookRepositoryImpl implements WebhookRepository
var _ WebhookRepository = (*WebhookRepositoryImpl)(nil)

type WebhookRepositoryImpl struct {
	db *sqlx.DB
}

func NewWebhookRepository(db *sqlx.DB) *WebhookRepositoryImpl {
	return &WebhookRepositoryImpl{db: db}
}

const webhookSelect = `SELECT id, user_id, group_id, name, url, event, secret, is_active, status,
	consecutive_failures, failing_since, last_delivery_at, created_at, updated_at FROM webhooks`

const deliverySelect = `SELECT id, webhook_id, event_id, status_code, attempt, delivered_at, error FROM webhook_deliveries`

func (r *WebhookRepositoryImpl) Create(ctx context.Context, w *model.Webhook) error {
	if w.Status == "" {
		w.Status = model.WebhookStatusActive
	}
	if w.Event == "" {
		w.Event = "engagement"
	}
	// A new webhook is active unless the caller says otherwise via Status.
	isActive := w.Status != model.WebhookStatusDeactivated
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO webhooks (user_id, group_id, name, url, event, secret, is_active, status)
		 VALUES (?, ?, ?, ?, ?, ?, ?, ?)`,
		w.UserID, w.GroupID, w.Name, w.URL, w.Event, w.Secret, isActive, w.Status)
	if err != nil {
		logger.Error(ctx, "webhook-repo: failed to create webhook", zap.Uint64("group_id", w.GroupID), zap.Error(err))
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	created, err := r.GetByID(ctx, uint64(id))
	if err != nil {
		return err
	}
	*w = *created
	return nil
}

func (r *WebhookRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.Webhook, error) {
	var w model.Webhook
	err := r.db.GetContext(ctx, &w, webhookSelect+` WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrWebhookNotFound
	}
	if err != nil {
		logger.Error(ctx, "webhook-repo: failed to get webhook", zap.Uint64("webhook_id", id), zap.Error(err))
		return nil, err
	}
	return &w, nil
}

func (r *WebhookRepositoryImpl) list(ctx context.Context, where string, groupID uint64) ([]model.Webhook, error) {
	rows := []model.Webhook{}
	if err := r.db.SelectContext(ctx, &rows, webhookSelect+` WHERE `+where+` ORDER BY created_at DESC, id DESC`, groupID); err != nil {
		logger.Error(ctx, "webhook-repo: failed to list webhooks", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

func (r *WebhookRepositoryImpl) ListByGroup(ctx context.Context, groupID uint64) ([]model.Webhook, error) {
	return r.list(ctx, `group_id = ?`, groupID)
}

func (r *WebhookRepositoryImpl) ListActiveByGroup(ctx context.Context, groupID uint64) ([]model.Webhook, error) {
	return r.list(ctx, `group_id = ? AND is_active = 1 AND status <> 'deactivated'`, groupID)
}

func (r *WebhookRepositoryImpl) Update(ctx context.Context, w *model.Webhook) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE webhooks SET name = ?, url = ?, is_active = ?, status = ?, consecutive_failures = ?,
		 failing_since = ?, last_delivery_at = ? WHERE id = ?`,
		w.Name, w.URL, w.IsActive, w.Status, w.ConsecutiveFailures,
		utcNullTime(w.FailingSince), utcNullTime(w.LastDeliveryAt), w.ID)
	if err != nil {
		logger.Error(ctx, "webhook-repo: failed to update webhook", zap.Uint64("webhook_id", w.ID), zap.Error(err))
		return err
	}
	return requireRowAffected(ctx, r.db, result, "webhooks", w.ID, ErrWebhookNotFound)
}

// utcNullTime is the DATETIME parameter for t (UTC), or NULL.
func utcNullTime(t model.NullTime) any {
	if !t.Valid {
		return nil
	}
	return t.Time.UTC()
}

func (r *WebhookRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM webhooks WHERE id = ?`, id)
	if err != nil {
		logger.Error(ctx, "webhook-repo: failed to delete webhook", zap.Uint64("webhook_id", id), zap.Error(err))
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrWebhookNotFound
	}
	return nil
}

// AddDelivery stores d (DeliveredAt defaults to now, Attempt to 1) and fills
// its ID and DeliveredAt, then prunes the webhook's older rows beyond the
// newest webhookDeliveriesKept (by id).
func (r *WebhookRepositoryImpl) AddDelivery(ctx context.Context, d *model.WebhookDelivery) error {
	if d.Attempt < 1 {
		d.Attempt = 1
	}
	var deliveredAt any = d.DeliveredAt.UTC()
	if d.DeliveredAt.IsZero() {
		deliveredAt = nil
	}
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO webhook_deliveries (webhook_id, event_id, status_code, attempt, delivered_at, error)
		 VALUES (?, ?, ?, ?, COALESCE(?, CURRENT_TIMESTAMP), ?)`,
		d.WebhookID, d.EventID, d.StatusCode, d.Attempt, deliveredAt, truncateRunes(d.Error, 500))
	if err != nil {
		logger.Error(ctx, "webhook-repo: failed to add delivery", zap.Uint64("webhook_id", d.WebhookID), zap.Error(err))
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	if err := r.db.GetContext(ctx, d, deliverySelect+` WHERE id = ?`, id); err != nil {
		logger.Error(ctx, "webhook-repo: failed to read delivery", zap.Uint64("delivery_id", uint64(id)), zap.Error(err))
		return err
	}
	// The derived table works around "LIMIT & IN subquery" not being
	// supported directly.
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM webhook_deliveries WHERE webhook_id = ? AND id NOT IN
		 (SELECT id FROM (SELECT id FROM webhook_deliveries WHERE webhook_id = ? ORDER BY id DESC LIMIT ?) t)`,
		d.WebhookID, d.WebhookID, webhookDeliveriesKept); err != nil {
		logger.Error(ctx, "webhook-repo: failed to prune deliveries", zap.Uint64("webhook_id", d.WebhookID), zap.Error(err))
		return err
	}
	return nil
}

// ListDeliveries orders by id (insertion order), the same order the pruning
// in AddDelivery uses.
func (r *WebhookRepositoryImpl) ListDeliveries(ctx context.Context, webhookID uint64, limit int) ([]model.WebhookDelivery, error) {
	rows := []model.WebhookDelivery{}
	if err := r.db.SelectContext(ctx, &rows, deliverySelect+` WHERE webhook_id = ? ORDER BY id DESC LIMIT ?`, webhookID, limit); err != nil {
		logger.Error(ctx, "webhook-repo: failed to list deliveries", zap.Uint64("webhook_id", webhookID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}
