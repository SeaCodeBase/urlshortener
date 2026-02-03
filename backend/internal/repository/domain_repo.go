package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var (
	ErrDomainNotFound = errors.New("domain not found")
	ErrDomainExists   = errors.New("domain already exists")
)

// Compile-time check: DomainRepositoryImpl implements DomainRepository
var _ DomainRepository = (*DomainRepositoryImpl)(nil)

type DomainRepositoryImpl struct {
	db *sqlx.DB
}

func NewDomainRepository(db *sqlx.DB) *DomainRepositoryImpl {
	return &DomainRepositoryImpl{db: db}
}

func (r *DomainRepositoryImpl) Create(ctx context.Context, domain *model.Domain) error {
	query := `INSERT INTO domains (user_id, domain) VALUES (?, ?)`
	result, err := r.db.ExecContext(ctx, query, domain.UserID, domain.Domain)
	if err != nil {
		if strings.Contains(err.Error(), "Duplicate entry") {
			return ErrDomainExists
		}
		logger.Error(ctx, "domain-repo: failed to create domain",
			zap.String("domain", domain.Domain),
			zap.Error(err),
		)
		return err
	}

	id, err := result.LastInsertId()
	if err != nil {
		logger.Error(ctx, "domain-repo: failed to get last insert ID",
			zap.Error(err),
		)
		return err
	}
	domain.ID = uint64(id)
	return nil
}

func (r *DomainRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.Domain, error) {
	var domain model.Domain
	query := `SELECT id, user_id, domain, created_at FROM domains WHERE id = ?`
	err := r.db.GetContext(ctx, &domain, query, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDomainNotFound
	}
	if err != nil {
		logger.Error(ctx, "domain-repo: failed to get domain by ID",
			zap.Uint64("id", id),
			zap.Error(err),
		)
		return nil, err
	}
	return &domain, nil
}

func (r *DomainRepositoryImpl) GetByDomain(ctx context.Context, domainName string) (*model.Domain, error) {
	var domain model.Domain
	query := `SELECT id, user_id, domain, created_at FROM domains WHERE domain = ?`
	err := r.db.GetContext(ctx, &domain, query, domainName)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrDomainNotFound
	}
	if err != nil {
		logger.Error(ctx, "domain-repo: failed to get domain by name",
			zap.String("domain", domainName),
			zap.Error(err),
		)
		return nil, err
	}
	return &domain, nil
}

func (r *DomainRepositoryImpl) ListByUserID(ctx context.Context, userID uint64) ([]*model.Domain, error) {
	var domains []*model.Domain
	query := `SELECT id, user_id, domain, created_at FROM domains WHERE user_id = ? ORDER BY created_at DESC`
	err := r.db.SelectContext(ctx, &domains, query, userID)
	if err != nil {
		logger.Error(ctx, "domain-repo: failed to list domains by user ID",
			zap.Uint64("userID", userID),
			zap.Error(err),
		)
		return nil, err
	}
	return domains, nil
}

func (r *DomainRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	query := `DELETE FROM domains WHERE id = ?`
	result, err := r.db.ExecContext(ctx, query, id)
	if err != nil {
		logger.Error(ctx, "domain-repo: failed to delete domain",
			zap.Uint64("id", id),
			zap.Error(err),
		)
		return err
	}

	rows, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if rows == 0 {
		return ErrDomainNotFound
	}
	return nil
}
