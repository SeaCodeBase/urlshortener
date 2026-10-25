package repository

import (
	"context"
	"database/sql"
	"errors"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"github.com/jmoiron/sqlx"
	"go.uber.org/zap"
)

var ErrBulkUploadNotFound = errors.New("bulk upload not found")

// Compile-time check: BulkUploadRepositoryImpl implements BulkUploadRepository
var _ BulkUploadRepository = (*BulkUploadRepositoryImpl)(nil)

type BulkUploadRepositoryImpl struct {
	db *sqlx.DB
}

func NewBulkUploadRepository(db *sqlx.DB) *BulkUploadRepositoryImpl {
	return &BulkUploadRepositoryImpl{db: db}
}

// bulkListColumns leaves out source_csv and result_csv (MEDIUMTEXT bodies).
const bulkListColumns = `id, user_id, group_id, filename, status, total, processed, succeeded, failed,
	error, created_at, updated_at`

const bulkSelect = `SELECT ` + bulkListColumns + `, source_csv, result_csv FROM bulk_uploads`

func (r *BulkUploadRepositoryImpl) Create(ctx context.Context, b *model.BulkUpload) error {
	result, err := r.db.ExecContext(ctx,
		`INSERT INTO bulk_uploads (user_id, group_id, filename, status, total, source_csv) VALUES (?, ?, ?, ?, ?, ?)`,
		b.UserID, b.GroupID, b.Filename, model.BulkStatusQueued, b.Total, b.SourceCSV)
	if err != nil {
		logger.Error(ctx, "bulk-repo: failed to create upload", zap.Uint64("group_id", b.GroupID), zap.Error(err))
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
	*b = *created
	return nil
}

func (r *BulkUploadRepositoryImpl) GetByID(ctx context.Context, id uint64) (*model.BulkUpload, error) {
	var b model.BulkUpload
	err := r.db.GetContext(ctx, &b, bulkSelect+` WHERE id = ?`, id)
	if errors.Is(err, sql.ErrNoRows) {
		return nil, ErrBulkUploadNotFound
	}
	if err != nil {
		logger.Error(ctx, "bulk-repo: failed to get upload", zap.Uint64("bulk_id", id), zap.Error(err))
		return nil, err
	}
	return &b, nil
}

func (r *BulkUploadRepositoryImpl) ListByGroup(ctx context.Context, groupID uint64) ([]model.BulkUpload, error) {
	rows := []model.BulkUpload{}
	if err := r.db.SelectContext(ctx, &rows,
		`SELECT `+bulkListColumns+` FROM bulk_uploads WHERE group_id = ? ORDER BY created_at DESC, id DESC`, groupID); err != nil {
		logger.Error(ctx, "bulk-repo: failed to list uploads", zap.Uint64("group_id", groupID), zap.Error(err))
		return nil, err
	}
	return rows, nil
}

// ClaimNext marks the oldest queued upload processing in a single UPDATE
// (concurrent claimers never get the same row) and reads its id back through
// LAST_INSERT_ID(expr), which is per connection: both statements run in one
// transaction, hence on one connection.
func (r *BulkUploadRepositoryImpl) ClaimNext(ctx context.Context) (*model.BulkUpload, error) {
	tx, err := r.db.BeginTxx(ctx, nil)
	if err != nil {
		logger.Error(ctx, "bulk-repo: failed to begin claim", zap.Error(err))
		return nil, err
	}
	defer func() { _ = tx.Rollback() }()
	result, err := tx.ExecContext(ctx,
		`UPDATE bulk_uploads SET id = LAST_INSERT_ID(id), status = ?
		 WHERE status = ? ORDER BY created_at, id LIMIT 1`,
		model.BulkStatusProcessing, model.BulkStatusQueued)
	if err != nil {
		logger.Error(ctx, "bulk-repo: failed to claim upload", zap.Error(err))
		return nil, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return nil, err
	}
	if n == 0 {
		return nil, nil
	}
	var id uint64
	if err := tx.GetContext(ctx, &id, `SELECT LAST_INSERT_ID()`); err != nil {
		logger.Error(ctx, "bulk-repo: failed to read claimed id", zap.Error(err))
		return nil, err
	}
	if err := tx.Commit(); err != nil {
		logger.Error(ctx, "bulk-repo: failed to commit claim", zap.Uint64("bulk_id", id), zap.Error(err))
		return nil, err
	}
	return r.GetByID(ctx, id)
}

// ReclaimStale requeues processing uploads whose updated_at is older than
// olderThan (the worker bumps it after every batch), resetting the counters
// so the rerun reports from scratch. The cutoff is computed by the database
// so it compares against the same clock that wrote updated_at.
func (r *BulkUploadRepositoryImpl) ReclaimStale(ctx context.Context, olderThan time.Duration) (int, error) {
	result, err := r.db.ExecContext(ctx,
		`UPDATE bulk_uploads SET status = ?, processed = 0, succeeded = 0, failed = 0
		 WHERE status = ? AND updated_at < NOW() - INTERVAL ? SECOND`,
		model.BulkStatusQueued, model.BulkStatusProcessing, int64(olderThan/time.Second))
	if err != nil {
		logger.Error(ctx, "bulk-repo: failed to reclaim stale uploads", zap.Error(err))
		return 0, err
	}
	n, err := result.RowsAffected()
	if err != nil {
		return 0, err
	}
	return int(n), nil
}

func (r *BulkUploadRepositoryImpl) UpdateProgress(ctx context.Context, id uint64, processed, succeeded, failed int) error {
	result, err := r.db.ExecContext(ctx,
		`UPDATE bulk_uploads SET processed = ?, succeeded = ?, failed = ? WHERE id = ?`,
		processed, succeeded, failed, id)
	if err != nil {
		logger.Error(ctx, "bulk-repo: failed to update progress", zap.Uint64("bulk_id", id), zap.Error(err))
		return err
	}
	return requireRowAffected(ctx, r.db, result, "bulk_uploads", id, ErrBulkUploadNotFound)
}

func (r *BulkUploadRepositoryImpl) Finish(ctx context.Context, id uint64, status string, resultCSV string, errMsg string) error {
	var res any = resultCSV
	if resultCSV == "" {
		res = nil
	}
	result, err := r.db.ExecContext(ctx,
		`UPDATE bulk_uploads SET status = ?, result_csv = ?, error = ? WHERE id = ?`,
		status, res, truncateRunes(errMsg, 500), id)
	if err != nil {
		logger.Error(ctx, "bulk-repo: failed to finish upload", zap.Uint64("bulk_id", id), zap.Error(err))
		return err
	}
	return requireRowAffected(ctx, r.db, result, "bulk_uploads", id, ErrBulkUploadNotFound)
}

// truncateRunes cuts s to at most n characters (VARCHAR(n) counts characters).
func truncateRunes(s string, n int) string {
	r := []rune(s)
	if len(r) <= n {
		return s
	}
	return string(r[:n])
}
