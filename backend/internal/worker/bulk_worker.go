package worker

import (
	"context"
	"errors"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/internal/service"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

const (
	// bulkStaleAfter is how long a processing upload may go without a
	// progress update before it is requeued (the worker died mid-job).
	bulkStaleAfter = 10 * time.Minute
	// bulkRowErrorMax caps a row's error message in the results CSV.
	bulkRowErrorMax = 200

	bulkMsgUnknownDomain = "unknown or unverified domain"
	bulkMsgInternal      = "internal error"
	bulkMsgShutdown      = "interrupted by server shutdown"
)

// BulkWorker processes queued bulk uploads: it creates one link per CSV row
// through the LinkService (so validation, tags and activity logging match
// POST /links), reports progress after every batch and stores a per-row
// results CSV. A failing row never aborts the upload.
type BulkWorker struct {
	bulk      repository.BulkUploadRepository
	links     service.LinkService
	domains   repository.DomainRepository
	pollEvery time.Duration
	batchSize int
	baseURL   string
	stopCh    chan struct{}
	doneCh    chan struct{}
}

func NewBulkWorker(bulk repository.BulkUploadRepository, linkSvc service.LinkService, domains repository.DomainRepository,
	pollEvery time.Duration, batchSize int, baseURL string) *BulkWorker {
	if pollEvery <= 0 {
		pollEvery = 5 * time.Second
	}
	if batchSize <= 0 {
		batchSize = 50
	}
	return &BulkWorker{
		bulk: bulk, links: linkSvc, domains: domains, pollEvery: pollEvery, batchSize: batchSize, baseURL: baseURL,
		stopCh: make(chan struct{}), doneCh: make(chan struct{}),
	}
}

func (w *BulkWorker) Start() {
	go w.run()
}

// Stop ends the loop after the current batch. An upload interrupted that way
// is finished as failed with the results so far, so a restart never creates
// its links twice.
func (w *BulkWorker) Stop() {
	close(w.stopCh)
	<-w.doneCh
}

func (w *BulkWorker) run() {
	defer close(w.doneCh)
	ticker := time.NewTicker(w.pollEvery)
	defer ticker.Stop()
	for {
		select {
		case <-ticker.C:
			w.poll()
		case <-w.stopCh:
			return
		}
	}
}

func (w *BulkWorker) stopping() bool {
	select {
	case <-w.stopCh:
		return true
	default:
		return false
	}
}

// poll requeues stale uploads, then processes queued ones until none is left.
func (w *BulkWorker) poll() {
	ctx := context.Background()
	if n, err := w.bulk.ReclaimStale(ctx, bulkStaleAfter); err != nil {
		logger.Error(ctx, "bulk-worker: failed to reclaim stale uploads", zap.Error(err))
	} else if n > 0 {
		logger.Warn(ctx, "bulk-worker: requeued stale uploads", zap.Int("uploads", n))
	}
	for !w.stopping() {
		job, err := w.bulk.ClaimNext(ctx)
		if err != nil {
			logger.Error(ctx, "bulk-worker: failed to claim upload", zap.Error(err))
			return
		}
		if job == nil {
			return
		}
		w.process(ctx, job)
	}
}

// process runs one claimed upload to completion; a panic fails the upload.
func (w *BulkWorker) process(ctx context.Context, job *model.BulkUpload) {
	defer func() {
		if r := recover(); r != nil {
			logger.Error(ctx, "bulk-worker: panic while processing upload",
				zap.Uint64("bulk_id", job.ID), zap.Any("panic", r))
			w.finish(ctx, job.ID, model.BulkStatusFailed, "", bulkMsgInternal)
		}
	}()

	rows, err := service.ParseBulkCSV(strings.NewReader(job.SourceCSV))
	if err != nil {
		w.finish(ctx, job.ID, model.BulkStatusFailed, "", err.Error())
		return
	}
	domains, err := w.domains.ListByUserID(ctx, job.UserID)
	if err != nil {
		logger.Error(ctx, "bulk-worker: failed to load domains", zap.Uint64("bulk_id", job.ID), zap.Error(err))
		w.finish(ctx, job.ID, model.BulkStatusFailed, "", bulkMsgInternal)
		return
	}
	verified := map[string]*model.Domain{}
	names := map[uint64]string{}
	for _, d := range domains {
		names[d.ID] = d.Domain
		if d.VerificationStatus == model.DomainVerificationOK {
			verified[strings.ToLower(d.Domain)] = d
		}
	}

	results := make([]service.BulkResultRow, 0, len(rows))
	succeeded, failed := 0, 0
	for start := 0; start < len(rows); start += w.batchSize {
		if start > 0 && w.stopping() {
			w.finishWithResults(ctx, job.ID, model.BulkStatusFailed, results, bulkMsgShutdown)
			return
		}
		end := min(start+w.batchSize, len(rows))
		for _, row := range rows[start:end] {
			res := w.createRow(ctx, job, row, verified, names)
			if res.Status == service.BulkRowOK {
				succeeded++
			} else {
				failed++
			}
			results = append(results, res)
		}
		if err := w.bulk.UpdateProgress(ctx, job.ID, len(results), succeeded, failed); err != nil {
			logger.Error(ctx, "bulk-worker: failed to update progress", zap.Uint64("bulk_id", job.ID), zap.Error(err))
			if errors.Is(err, repository.ErrBulkUploadNotFound) {
				return
			}
			w.finishWithResults(ctx, job.ID, model.BulkStatusFailed, results, bulkMsgInternal)
			return
		}
	}
	w.finishWithResults(ctx, job.ID, model.BulkStatusDone, results, "")
	logger.Info(ctx, "bulk-worker: upload processed", zap.Uint64("bulk_id", job.ID),
		zap.Int("succeeded", succeeded), zap.Int("failed", failed))
}

func (w *BulkWorker) createRow(ctx context.Context, job *model.BulkUpload, row service.BulkRow,
	verified map[string]*model.Domain, names map[uint64]string) service.BulkResultRow {
	res := service.BulkResultRow{Row: row.Line, LongURL: row.LongURL, Status: service.BulkRowError}
	var domainID *uint64
	if row.Domain != "" {
		d, ok := verified[strings.ToLower(row.Domain)]
		if !ok {
			res.Error = bulkMsgUnknownDomain
			return res
		}
		id := d.ID
		domainID = &id
	}
	link, err := w.links.Create(ctx, job.UserID, service.CreateLinkInput{
		OriginalURL: row.LongURL,
		Title:       row.Title,
		CustomCode:  row.CustomBackHalf,
		DomainID:    domainID,
		Tags:        row.Tags,
		GroupID:     job.GroupID,
	})
	if err != nil {
		res.Error = bulkRowMessage(ctx, job.ID, row.Line, err)
		return res
	}
	domainName := ""
	if link.DomainID != nil {
		domainName = w.domainName(ctx, *link.DomainID, names)
	}
	res.Status = service.BulkRowOK
	res.ShortURL = service.ShortURL(w.baseURL, link, domainName)
	return res
}

// domainName resolves a link's domain (the group default may not be in the
// user's list, e.g. after it was reassigned) and caches it in names.
func (w *BulkWorker) domainName(ctx context.Context, id uint64, names map[uint64]string) string {
	if n, ok := names[id]; ok {
		return n
	}
	d, err := w.domains.GetByID(ctx, id)
	if err != nil {
		logger.Warn(ctx, "bulk-worker: failed to load link domain", zap.Uint64("domain_id", id), zap.Error(err))
		return ""
	}
	names[id] = d.Domain
	return d.Domain
}

// bulkRowMessage turns a link creation error into a message safe to show in
// the results CSV: validation and known sentinel errors keep a readable
// message, anything else is logged and reported generically.
func bulkRowMessage(ctx context.Context, bulkID uint64, line int, err error) string {
	var ve *service.ValidationError
	var msg string
	switch {
	case errors.As(err, &ve):
		msg = ve.Msg
	case errors.Is(err, service.ErrShortCodeTaken):
		msg = "that back-half is already taken"
	case errors.Is(err, service.ErrReservedShortCode):
		msg = "this back-half is reserved"
	case errors.Is(err, service.ErrInvalidShortCode):
		msg = "back-halves can only contain letters, numbers, hyphens and underscores (1-255 characters)"
	case errors.Is(err, service.ErrInvalidURL):
		msg = "destination must be a valid http(s) URL of at most 2048 characters"
	case errors.Is(err, service.ErrDomainNotFound), errors.Is(err, service.ErrDomainNotVerified):
		msg = bulkMsgUnknownDomain
	case errors.Is(err, service.ErrInvalidUTMParam), errors.Is(err, service.ErrExpirationTooSoon),
		errors.Is(err, service.ErrInvalidRoutingRule):
		msg = err.Error()
	default:
		logger.Error(ctx, "bulk-worker: failed to create link", zap.Uint64("bulk_id", bulkID),
			zap.Int("row", line), zap.Error(err))
		msg = "failed to create link"
	}
	if r := []rune(msg); len(r) > bulkRowErrorMax {
		msg = string(r[:bulkRowErrorMax])
	}
	return msg
}

func (w *BulkWorker) finishWithResults(ctx context.Context, id uint64, status string, results []service.BulkResultRow, errMsg string) {
	out, err := service.EncodeBulkResults(results)
	if err != nil {
		logger.Error(ctx, "bulk-worker: failed to encode results", zap.Uint64("bulk_id", id), zap.Error(err))
		w.finish(ctx, id, model.BulkStatusFailed, "", bulkMsgInternal)
		return
	}
	w.finish(ctx, id, status, out, errMsg)
}

func (w *BulkWorker) finish(ctx context.Context, id uint64, status, resultCSV, errMsg string) {
	if err := w.bulk.Finish(ctx, id, status, resultCSV, errMsg); err != nil {
		logger.Error(ctx, "bulk-worker: failed to finish upload", zap.Uint64("bulk_id", id),
			zap.String("status", status), zap.Error(err))
	}
}
