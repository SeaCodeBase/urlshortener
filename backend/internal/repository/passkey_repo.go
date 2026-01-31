package repository

import (
	"context"
	"database/sql"
	"errors"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/jmoiron/sqlx"
)

var ErrPasskeyNotFound = errors.New("passkey not found")

type passkeyRepo struct {
	db *sqlx.DB
}

func NewPasskeyRepository(db *sqlx.DB) PasskeyRepository {
	return &passkeyRepo{db: db}
}

func (r *passkeyRepo) Create(ctx context.Context, passkey *model.Passkey) error {
	query := `INSERT INTO passkeys (user_id, name, credential_id, public_key, counter)
	          VALUES (?, ?, ?, ?, ?)`
	result, err := r.db.ExecContext(ctx, query, passkey.UserID, passkey.Name, passkey.CredentialID, passkey.PublicKey, passkey.Counter)
	if err != nil {
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	passkey.ID = uint64(id)
	return nil
}

func (r *passkeyRepo) GetByCredentialID(ctx context.Context, credentialID []byte) (*model.Passkey, error) {
	var passkey model.Passkey
	query := `SELECT id, user_id, name, credential_id, public_key, counter, created_at, last_used_at
	          FROM passkeys WHERE credential_id = ?`
	err := r.db.GetContext(ctx, &passkey, query, credentialID)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrPasskeyNotFound
	}
	return &passkey, err
}

func (r *passkeyRepo) ListByUserID(ctx context.Context, userID uint64) ([]model.Passkey, error) {
	var passkeys []model.Passkey
	query := `SELECT id, user_id, name, credential_id, public_key, counter, created_at, last_used_at
	          FROM passkeys WHERE user_id = ? ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &passkeys, query, userID)
	if err != nil {
		return nil, err
	}
	if passkeys == nil {
		passkeys = []model.Passkey{}
	}
	return passkeys, nil
}

func (r *passkeyRepo) UpdateCounter(ctx context.Context, id uint64, counter uint32) error {
	query := `UPDATE passkeys SET counter = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, counter, id)
	return err
}

func (r *passkeyRepo) UpdateLastUsedAt(ctx context.Context, id uint64) error {
	query := `UPDATE passkeys SET last_used_at = NOW() WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *passkeyRepo) UpdateName(ctx context.Context, id uint64, name string) error {
	query := `UPDATE passkeys SET name = ? WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, name, id)
	return err
}

func (r *passkeyRepo) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM passkeys WHERE id = ?`
	_, err := r.db.ExecContext(ctx, query, id)
	return err
}

func (r *passkeyRepo) CountByUserID(ctx context.Context, userID uint64) (int, error) {
	var count int
	query := `SELECT COUNT(*) FROM passkeys WHERE user_id = ?`
	err := r.db.GetContext(ctx, &count, query, userID)
	return count, err
}
