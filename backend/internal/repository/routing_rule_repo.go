package repository

import (
	"context"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

// Compile-time check: RoutingRuleRepositoryImpl implements RoutingRuleRepository
var _ RoutingRuleRepository = (*RoutingRuleRepositoryImpl)(nil)

type RoutingRuleRepositoryImpl struct {
	db *sqlx.DB
}

func NewRoutingRuleRepository(db *sqlx.DB) *RoutingRuleRepositoryImpl {
	return &RoutingRuleRepositoryImpl{db: db}
}

func (r *RoutingRuleRepositoryImpl) ListByLinkID(ctx context.Context, linkID uint64) ([]model.RoutingRule, error) {
	rules := []model.RoutingRule{}
	query := `SELECT id, link_id, position, long_url, device_match, os_match, country_match, country_exclude
			  FROM dynamic_routing_rules WHERE link_id = ? ORDER BY position, id`
	if err := r.db.SelectContext(ctx, &rules, query, linkID); err != nil {
		logger.Error(ctx, "routing-rule-repo: failed to list rules",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return nil, err
	}
	return rules, nil
}

func (r *RoutingRuleRepositoryImpl) ReplaceForLink(ctx context.Context, linkID uint64, rules []model.RoutingRule) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		return err
	}
	defer func() { _ = tx.Rollback() }()

	if _, err := tx.ExecContext(ctx, `DELETE FROM dynamic_routing_rules WHERE link_id = ?`, linkID); err != nil {
		logger.Error(ctx, "routing-rule-repo: failed to delete rules",
			zap.Uint64("link_id", linkID),
			zap.Error(err),
		)
		return err
	}

	insert := `INSERT INTO dynamic_routing_rules (link_id, position, long_url, device_match, os_match, country_match, country_exclude)
			   VALUES (?, ?, ?, ?, ?, ?, ?)`
	for i := range rules {
		rule := &rules[i]
		rule.LinkID = linkID
		rule.Position = i
		res, err := tx.ExecContext(ctx, insert, linkID, i, rule.LongURL,
			rule.DeviceMatch, rule.OSMatch, rule.CountryMatch, rule.CountryExclude)
		if err != nil {
			logger.Error(ctx, "routing-rule-repo: failed to insert rule",
				zap.Uint64("link_id", linkID),
				zap.Int("position", i),
				zap.Error(err),
			)
			return err
		}
		if id, err := res.LastInsertId(); err == nil {
			rule.ID = uint64(id)
		}
	}
	return tx.Commit()
}
