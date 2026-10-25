package repository

import (
	"context"
	"database/sql"
	"errors"
	"strings"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/go-sql-driver/mysql"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var (
	ErrCampaignNotFound = errors.New("campaign not found")
	ErrChannelNotFound  = errors.New("channel not found")
	ErrChannelExists    = errors.New("channel already exists")
)

// Compile-time check: CampaignRepositoryImpl implements CampaignRepository
var _ CampaignRepository = (*CampaignRepositoryImpl)(nil)

type CampaignRepositoryImpl struct {
	db *sqlx.DB
}

func NewCampaignRepository(db *sqlx.DB) *CampaignRepositoryImpl {
	return &CampaignRepositoryImpl{db: db}
}

const campaignColumns = `c.id, c.user_id, c.group_id, c.name, c.description, c.auto_utm, c.created_at, c.updated_at`

const channelSelect = `SELECT ch.id, ch.group_id, ch.name, ch.created_at FROM channels ch`

const campaignLinkSelect = `SELECT campaign_id, channel_id, link_id, created_at FROM campaign_links`

// requireRowAffected maps 0 affected rows to notFound unless the row with
// that id exists in table (an UPDATE that changes nothing affects 0 rows).
func requireRowAffected(ctx context.Context, db sqlx.QueryerContext, result sql.Result, table string, id uint64, notFound error) error {
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n > 0 {
		return nil
	}
	var exists int
	if err := sqlx.GetContext(ctx, db, &exists, `SELECT COUNT(*) FROM `+table+` WHERE id = ?`, id); err != nil {
		return err
	}
	if exists == 0 {
		return notFound
	}
	return nil
}

func (r *CampaignRepositoryImpl) Create(ctx context.Context, c *model.Campaign) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO campaigns (user_id, group_id, name, description, auto_utm) VALUES (?, ?, ?, ?, ?)`,
		c.UserID, c.GroupID, c.Name, c.Description, c.AutoUTM)
	if err != nil {
		logger.Error(ctx, "campaign-repo: failed to create campaign", zap.Uint64("group_id", c.GroupID), zap.Error(err))
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
	*c = *created
	return nil
}

func (r *CampaignRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.Campaign, error) {
	var c model.Campaign
	err := r.db.GetContext(ctx, &c, `SELECT `+campaignColumns+` FROM campaigns c WHERE c.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrCampaignNotFound
	}
	if err != nil {
		logger.Error(ctx, "campaign-repo: failed to get campaign", zap.Uint64("campaign_id", id), zap.Error(err))
		return nil, err
	}
	return &c, nil
}

func (r *CampaignRepositoryImpl) ListByGroup(ctx context.Context, groupID uint64) ([]model.CampaignSummary, error) {
	// Pre-aggregated LEFT JOINs: no channel x link fan-out and no GROUP BY
	// over the campaign columns.
	query := `SELECT ` + campaignColumns + `,
			COALESCE(chs.n, 0) AS channel_count, COALESCE(lks.n, 0) AS link_count
		FROM campaigns c
		LEFT JOIN (SELECT campaign_id, COUNT(DISTINCT channel_id) AS n FROM campaign_channels GROUP BY campaign_id) chs
			ON chs.campaign_id = c.id
		LEFT JOIN (SELECT campaign_id, COUNT(DISTINCT link_id) AS n FROM campaign_links GROUP BY campaign_id) lks
			ON lks.campaign_id = c.id
		WHERE c.group_id = ?
		ORDER BY c.created_at DESC, c.id DESC`
	rows := []model.CampaignSummary{}
	if err := r.db.SelectContext(ctx, &rows, query, groupID); err != nil {
		logger.Error(ctx, "campaign-repo: failed to list campaigns", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

func (r *CampaignRepositoryImpl) Update(ctx context.Context, c *model.Campaign) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE campaigns SET name = ?, description = ?, auto_utm = ? WHERE id = ?`,
		c.Name, c.Description, c.AutoUTM, c.ID)
	if err != nil {
		logger.Error(ctx, "campaign-repo: failed to update campaign", zap.Uint64("campaign_id", c.ID), zap.Error(err))
		return err
	}
	return requireRowAffected(ctx, r.db, result, "campaigns", c.ID, ErrCampaignNotFound)
}

func (r *CampaignRepositoryImpl) Delete(ctx context.Context, id uint64) error {
	result, err := r.db.ExecContext(ctx, `DELETE FROM campaigns WHERE id = ?`, id)
	if err != nil {
		logger.Error(ctx, "campaign-repo: failed to delete campaign", zap.Uint64("campaign_id", id), zap.Error(err))
		return err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return err
	}
	if n == 0 {
		return ErrCampaignNotFound
	}
	return nil
}

func (r *CampaignRepositoryImpl) CreateChannel(ctx context.Context, ch *model.Channel) error {
	result, err := r.db.ExecContext(ctx, `INSERT INTO channels (group_id, name) VALUES (?, ?)`, ch.GroupID, ch.Name)
	if err != nil {
		var me *mysql.MySQLError
		if errors.As(err, &me) && me.Number == mysqlDuplicateEntry {
			return ErrChannelExists
		}
		logger.Error(ctx, "campaign-repo: failed to create channel", zap.Uint64("group_id", ch.GroupID), zap.Error(err))
		return err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return err
	}
	created, err := r.GetChannel(ctx, uint64(id))
	if err != nil {
		return err
	}
	*ch = *created
	return nil
}

func (r *CampaignRepositoryImpl) ListChannels(ctx context.Context, groupID uint64) ([]model.Channel, error) {
	rows := []model.Channel{}
	if err := r.db.SelectContext(ctx, &rows, channelSelect+` WHERE ch.group_id = ? ORDER BY ch.name, ch.id`, groupID); err != nil {
		logger.Error(ctx, "campaign-repo: failed to list channels", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

func (r *CampaignRepositoryImpl) GetChannel(ctx context.Context, id uint64) (*model.Channel, error) {
	var ch model.Channel
	err := r.db.GetContext(ctx, &ch, channelSelect+` WHERE ch.id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrChannelNotFound
	}
	if err != nil {
		logger.Error(ctx, "campaign-repo: failed to get channel", zap.Uint64("channel_id", id), zap.Error(err))
		return nil, err
	}
	return &ch, nil
}

func (r *CampaignRepositoryImpl) SetCampaignChannels(ctx context.Context, campaignID uint64, channelIDs []uint64) error {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error(ctx, "campaign-repo: failed to begin set channels", zap.Uint64("campaign_id", campaignID), zap.Error(err))
		return err
	}
	defer func() { _ = tx.Rollback() }()

	// Lock the campaign row first so concurrent replacements serialize.
	var exists int
	if err := tx.GetContext(ctx, &exists, `SELECT COUNT(*) FROM campaigns WHERE id = ? FOR UPDATE`, campaignID); err != nil {
		logger.Error(ctx, "campaign-repo: failed to lock campaign", zap.Uint64("campaign_id", campaignID), zap.Error(err))
		return err
	}
	if exists == 0 {
		return ErrCampaignNotFound
	}

	// Removing a campaign_channels row cascades to its campaign_links rows.
	del := `DELETE FROM campaign_channels WHERE campaign_id = ?`
	args := []any{campaignID}
	if len(channelIDs) > 0 {
		q, inArgs, err := sqlx.In(del+` AND channel_id NOT IN (?)`, campaignID, channelIDs)
		if err != nil {
			return err
		}
		del, args = tx.Rebind(q), inArgs
	}
	if _, err := tx.ExecContext(ctx, del, args...); err != nil {
		logger.Error(ctx, "campaign-repo: failed to remove channels", zap.Uint64("campaign_id", campaignID), zap.Error(err))
		return err
	}
	if len(channelIDs) > 0 {
		values := strings.TrimSuffix(strings.Repeat("(?, ?),", len(channelIDs)), ",")
		ins := make([]any, 0, 2*len(channelIDs))
		for _, id := range channelIDs {
			ins = append(ins, campaignID, id)
		}
		// ON DUPLICATE KEY (not INSERT IGNORE) keeps FK violations errors.
		if _, err := tx.ExecContext(ctx,
			`INSERT INTO campaign_channels (campaign_id, channel_id) VALUES `+values+
				` ON DUPLICATE KEY UPDATE channel_id = channel_id`, ins...); err != nil {
			logger.Error(ctx, "campaign-repo: failed to add channels", zap.Uint64("campaign_id", campaignID), zap.Error(err))
			return err
		}
	}
	if err := tx.Commit(); err != nil {
		logger.Error(ctx, "campaign-repo: failed to commit set channels", zap.Uint64("campaign_id", campaignID), zap.Error(err))
		return err
	}
	return nil
}

func (r *CampaignRepositoryImpl) CampaignChannels(ctx context.Context, campaignID uint64) ([]model.Channel, error) {
	rows := []model.Channel{}
	if err := r.db.SelectContext(ctx, &rows,
		channelSelect+` JOIN campaign_channels cc ON cc.channel_id = ch.id WHERE cc.campaign_id = ? ORDER BY ch.name, ch.id`,
		campaignID); err != nil {
		logger.Error(ctx, "campaign-repo: failed to list campaign channels", zap.Uint64("campaign_id", campaignID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

// AddLinks ignores existing memberships. It uses ON DUPLICATE KEY rather than
// INSERT IGNORE, which would also silently drop rows violating a foreign key
// (an unknown link or a channel not attached to the campaign); those return
// the driver error instead.
func (r *CampaignRepositoryImpl) AddLinks(ctx context.Context, campaignID, channelID uint64, linkIDs []uint64) error {
	if len(linkIDs) == 0 {
		return nil
	}
	values := strings.TrimSuffix(strings.Repeat("(?, ?, ?),", len(linkIDs)), ",")
	args := make([]any, 0, 3*len(linkIDs))
	for _, id := range linkIDs {
		args = append(args, campaignID, channelID, id)
	}
	if _, err := r.db.ExecContext(ctx,
		`INSERT INTO campaign_links (campaign_id, channel_id, link_id) VALUES `+values+
			` ON DUPLICATE KEY UPDATE link_id = link_id`, args...); err != nil {
		logger.Error(ctx, "campaign-repo: failed to add links",
			zap.Uint64("campaign_id", campaignID), zap.Uint64("channel_id", channelID), zap.Error(err))
		return err
	}
	return nil
}

func (r *CampaignRepositoryImpl) RemoveLink(ctx context.Context, campaignID, channelID, linkID uint64) error {
	if _, err := r.db.ExecContext(ctx,
		`DELETE FROM campaign_links WHERE campaign_id = ? AND channel_id = ? AND link_id = ?`,
		campaignID, channelID, linkID); err != nil {
		logger.Error(ctx, "campaign-repo: failed to remove link",
			zap.Uint64("campaign_id", campaignID), zap.Uint64("link_id", linkID), zap.Error(err))
		return err
	}
	return nil
}

func (r *CampaignRepositoryImpl) Links(ctx context.Context, campaignID uint64) ([]model.CampaignLink, error) {
	rows := []model.CampaignLink{}
	if err := r.db.SelectContext(ctx, &rows,
		campaignLinkSelect+` WHERE campaign_id = ? ORDER BY created_at DESC, link_id DESC, channel_id`, campaignID); err != nil {
		logger.Error(ctx, "campaign-repo: failed to list campaign links", zap.Uint64("campaign_id", campaignID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

func (r *CampaignRepositoryImpl) LinkCampaigns(ctx context.Context, linkID uint64) ([]model.CampaignLink, error) {
	rows := []model.CampaignLink{}
	if err := r.db.SelectContext(ctx, &rows,
		campaignLinkSelect+` WHERE link_id = ? ORDER BY campaign_id, channel_id`, linkID); err != nil {
		logger.Error(ctx, "campaign-repo: failed to list link campaigns", zap.Uint64("link_id", linkID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}
