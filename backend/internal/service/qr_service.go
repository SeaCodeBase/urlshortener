package service

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"regexp"
	"strconv"
	"strings"
	"time"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
	"github.com/SeaCodeBase/urlshortener/pkg/logger"
	"go.uber.org/zap"
)

var (
	ErrQRCodeNotFound      = errors.New("qr code not found")
	ErrQRCodeExistsForLink = errors.New("This link already has a QR Code")
	ErrQRLinkRequired      = errors.New("long_url or link_id is required")
	// ErrInvalidRenderCustomizations: the design is too large or has an
	// invalid error_correction/color/background value.
	ErrInvalidRenderCustomizations = errors.New("render_customizations must be a JSON object of at most 8 KB")
)

// maxRenderCustomizationsBytes bounds the serialised render_customizations.
const maxRenderCustomizationsBytes = 8192

var (
	hexColor         = regexp.MustCompile(`^#[0-9A-Fa-f]{6}$`)
	errorCorrections = map[string]bool{"L": true, "M": true, "Q": true, "H": true}
	// uploadedLogo is the path of an image stored by POST /uploads.
	uploadedLogo = regexp.MustCompile(`^/uploads/[0-9a-f]{32}\.(png|jpg)$`)
)

// QRExportHeader is the CSV header of the QR code export.
var QRExportHeader = []string{"Title", "Short Link", "Destination URL", "Date created", "Scans", "Status"}

// Compile-time check: QRServiceImpl implements QRService
var _ QRService = (*QRServiceImpl)(nil)

// QRServiceDeps are the collaborators of QRServiceImpl.
type QRServiceDeps struct {
	QRs   repository.QRRepository
	Links LinkService
	// LinkRepo deletes a QR-owned link (source 'qr') together with its QR
	// code, bypassing LinkService.Delete's deletability rule.
	LinkRepo repository.LinkRepository
	Activity ActivityService
	// Domains and BaseURL build short links for Export.
	Domains repository.DomainRepository
	BaseURL string
}

type QRServiceImpl struct {
	qrRepo     repository.QRRepository
	links      LinkService
	linkRepo   repository.LinkRepository
	domainRepo repository.DomainRepository
	activity   ActivityService
	baseURL    string
}

func NewQRService(d QRServiceDeps) *QRServiceImpl {
	return &QRServiceImpl{
		qrRepo:     d.QRs,
		links:      d.Links,
		linkRepo:   d.LinkRepo,
		domainRepo: d.Domains,
		activity:   d.Activity,
		baseURL:    strings.TrimRight(d.BaseURL, "/"),
	}
}

// CreateQRCodeInput creates a QR code for an existing link (LinkID) or for a
// new link to LongURL.
type CreateQRCodeInput struct {
	LinkID   *uint64 `json:"link_id"`
	LongURL  string  `json:"long_url"`
	DomainID *uint64 `json:"domain_id"`
	// ShareAsLink makes the new link a regular Links-list link (source
	// 'link'); CustomCode and Tags are only applied to the link then.
	ShareAsLink  bool               `json:"share_as_link"`
	CustomCode   string             `json:"custom_code"`
	Title        string             `json:"title"`
	Tags         []string           `json:"tags"`
	UTM          *UTMParams         `json:"utm"`
	ExpiresAt    *time.Time         `json:"expires_at"`
	RoutingRules []RoutingRuleInput `json:"routing_rules"`
	// RenderCustomizations nil stores {}.
	RenderCustomizations model.JSONMap `json:"render_customizations"`
	// GroupID is set by the handler from the active group, never from JSON.
	GroupID uint64 `json:"-"`
}

// UpdateQRCodeInput edits a QR code; nil fields are kept.
type UpdateQRCodeInput struct {
	// Title "" clears the QR title (the link title is then displayed).
	Title *string   `json:"title"`
	Tags  *[]string `json:"tags"`
	// RenderCustomizations nil keeps the design.
	RenderCustomizations model.JSONMap `json:"render_customizations"`
	LongURL              string        `json:"long_url"`
	ExpiresAt            *time.Time    `json:"expires_at"`
	// ClearExpiration removes the expiration (takes precedence over ExpiresAt).
	ClearExpiration bool                `json:"clear_expiration"`
	RoutingRules    *[]RoutingRuleInput `json:"routing_rules"`
}

// QRCodeDetails is a QR code with its link, tags and scan count.
type QRCodeDetails struct {
	model.QRCode
	Link       model.Link
	Tags       []string
	TotalScans int64
	// RoutingRules is only filled by Get.
	RoutingRules []model.RoutingRule
}

type ListQRParams struct {
	Filter repository.QRFilter
	Page   int
	Limit  int
}

type ListQRResult struct {
	QRCodes    []QRCodeDetails
	Total      int64
	Page       int
	TotalPages int
}

// normalizeRenderCustomizations returns {} for nil and validates a design:
// at most 8 KB serialised, error_correction in L|M|Q|H, color and background
// as #RRGGBB, logo null or an /uploads/ image path.
func normalizeRenderCustomizations(rc model.JSONMap) (model.JSONMap, error) {
	if rc == nil {
		return model.JSONMap{}, nil
	}
	b, err := json.Marshal(rc)
	if err != nil || len(b) > maxRenderCustomizationsBytes {
		return nil, ErrInvalidRenderCustomizations
	}
	if v, ok := rc["error_correction"]; ok {
		if s, ok := v.(string); !ok || !errorCorrections[s] {
			return nil, fmt.Errorf("%w: error_correction must be one of L, M, Q, H", ErrInvalidRenderCustomizations)
		}
	}
	for _, key := range []string{"color", "background"} {
		if v, ok := rc[key]; ok {
			if s, ok := v.(string); !ok || !hexColor.MatchString(s) {
				return nil, fmt.Errorf("%w: %s must be a #RRGGBB color", ErrInvalidRenderCustomizations, key)
			}
		}
	}
	if v, ok := rc["logo"]; ok && v != nil {
		if s, ok := v.(string); !ok || !uploadedLogo.MatchString(s) {
			return nil, fmt.Errorf("%w: logo must be an uploaded image path", ErrInvalidRenderCustomizations)
		}
	}
	return rc, nil
}

// qrTitle returns nil for a blank title, otherwise the trimmed title.
func qrTitle(title string) *string {
	if t := strings.TrimSpace(title); t != "" {
		return &t
	}
	return nil
}

// attachableLink returns the user's link linkID when a QR code may be
// created for it: in groupID, a Links-list link (source 'link') and without
// a QR code.
func (s *QRServiceImpl) attachableLink(ctx context.Context, userID, groupID, linkID uint64) (*model.Link, error) {
	link, err := s.links.GetByID(ctx, userID, linkID)
	if errors.Is(err, ErrNotLinkOwner) {
		return nil, ErrLinkNotFound
	}
	if err != nil {
		return nil, err
	}
	if link.GroupID != groupID || link.Source != model.LinkSourceLink {
		return nil, ErrLinkNotFound
	}
	_, err = s.qrRepo.GetByLinkID(ctx, link.ID)
	if err == nil {
		return nil, ErrQRCodeExistsForLink
	}
	if !errors.Is(err, repository.ErrQRCodeNotFound) {
		return nil, err
	}
	return link, nil
}

// Create creates a QR code for an existing link or for a new link.
func (s *QRServiceImpl) Create(ctx context.Context, userID uint64, input CreateQRCodeInput) (*QRCodeDetails, error) {
	if input.GroupID == 0 {
		return nil, ErrGroupRequired
	}
	rc, err := normalizeRenderCustomizations(input.RenderCustomizations)
	if err != nil {
		return nil, err
	}

	var link *model.Link
	createdLink := false
	switch {
	case input.LinkID != nil:
		if link, err = s.attachableLink(ctx, userID, input.GroupID, *input.LinkID); err != nil {
			return nil, err
		}
	case strings.TrimSpace(input.LongURL) != "":
		in := CreateLinkInput{
			OriginalURL:  input.LongURL,
			Title:        input.Title,
			ExpiresAt:    input.ExpiresAt,
			DomainID:     input.DomainID,
			UTM:          input.UTM,
			RoutingRules: input.RoutingRules,
			GroupID:      input.GroupID,
			Source:       model.LinkSourceQR,
		}
		if input.ShareAsLink {
			in.Source = model.LinkSourceLink
			in.CustomCode = input.CustomCode
			in.Tags = input.Tags
		}
		if link, err = s.links.Create(ctx, userID, in); err != nil {
			return nil, err
		}
		createdLink = true
	default:
		return nil, ErrQRLinkRequired
	}

	qr := &model.QRCode{
		UserID:               userID,
		GroupID:              input.GroupID,
		LinkID:               link.ID,
		Title:                qrTitle(input.Title),
		RenderCustomizations: rc,
	}
	tags := repository.NormalizeTags(input.Tags) // never nil
	if err := s.qrRepo.Create(ctx, qr); err != nil {
		s.rollbackCreate(ctx, 0, link, createdLink)
		if errors.Is(err, repository.ErrQRCodeExistsForLink) {
			return nil, ErrQRCodeExistsForLink
		}
		logger.Error(ctx, "qr-service: failed to create qr code",
			zap.Uint64("user_id", userID),
			zap.Uint64("link_id", link.ID),
			zap.Error(err),
		)
		return nil, err
	}
	if len(tags) > 0 {
		if err := s.qrRepo.SetTags(ctx, qr.ID, tags); err != nil {
			logger.Error(ctx, "qr-service: failed to save qr tags; removing qr code",
				zap.Uint64("qr_code_id", qr.ID),
				zap.Error(err),
			)
			s.rollbackCreate(ctx, qr.ID, link, createdLink)
			return nil, err
		}
	}

	s.activity.Log(detached(ctx), userID, qr.GroupID, ActionQRCreate, EntityQRCode, qr.ID,
		map[string]any{"short_code": link.ShortCode, "link_id": link.ID})
	return &QRCodeDetails{QRCode: *qr, Link: *link, Tags: tags}, nil
}

// rollbackCreate removes a partially created QR code (qrID 0: none) and the
// link created for it (when createdLink).
func (s *QRServiceImpl) rollbackCreate(ctx context.Context, qrID uint64, link *model.Link, createdLink bool) {
	ctx = detached(ctx)
	if qrID != 0 {
		if err := s.qrRepo.Delete(ctx, qrID); err != nil {
			logger.Error(ctx, "qr-service: failed to remove partially created qr code",
				zap.Uint64("qr_code_id", qrID), zap.Error(err))
		}
	}
	if createdLink {
		if err := s.linkRepo.Delete(ctx, link.ID); err != nil {
			logger.Error(ctx, "qr-service: failed to remove link of failed qr code",
				zap.Uint64("link_id", link.ID), zap.Error(err))
		}
	}
}

// getOwned loads the QR code and checks it belongs to userID.
func (s *QRServiceImpl) getOwned(ctx context.Context, userID, id uint64) (*repository.QRCodeRow, error) {
	row, err := s.qrRepo.GetByID(ctx, id)
	if errors.Is(err, repository.ErrQRCodeNotFound) {
		return nil, ErrQRCodeNotFound
	}
	if err != nil {
		return nil, err
	}
	if row.UserID != userID {
		return nil, ErrQRCodeNotFound
	}
	return row, nil
}

// tagsOf returns the QR code's tags, never nil.
func (s *QRServiceImpl) tagsOf(ctx context.Context, id uint64) ([]string, error) {
	byQR, err := s.qrRepo.GetTagsFor(ctx, []uint64{id})
	if err != nil {
		return nil, err
	}
	if tags, ok := byQR[id]; ok {
		return tags, nil
	}
	return []string{}, nil
}

// Get returns the user's QR code with its link, tags, scans and routing rules.
func (s *QRServiceImpl) Get(ctx context.Context, userID, id uint64) (*QRCodeDetails, error) {
	row, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return nil, err
	}
	d := &QRCodeDetails{QRCode: row.QRCode, Link: row.ToLink(), TotalScans: row.TotalScans}
	if d.Tags, err = s.tagsOf(ctx, row.ID); err != nil {
		return nil, err
	}
	if d.RoutingRules, err = s.links.GetRoutingRules(ctx, userID, row.LinkID); err != nil {
		return nil, err
	}
	return d, nil
}

// List pages through the group's QR codes matching params.Filter, with tags.
func (s *QRServiceImpl) List(ctx context.Context, groupID uint64, params ListQRParams) (*ListQRResult, error) {
	page, limit := normalizePaging(params.Page, params.Limit)
	rows, total, err := s.qrRepo.ListByGroup(ctx, groupID, params.Filter, limit, (page-1)*limit)
	if err != nil {
		return nil, err
	}
	ids := make([]uint64, len(rows))
	for i := range rows {
		ids[i] = rows[i].ID
	}
	byQR, err := s.qrRepo.GetTagsFor(ctx, ids)
	if err != nil {
		return nil, err
	}
	out := make([]QRCodeDetails, len(rows))
	for i := range rows {
		tags, ok := byQR[rows[i].ID]
		if !ok {
			tags = []string{}
		}
		out[i] = QRCodeDetails{QRCode: rows[i].QRCode, Link: rows[i].ToLink(), Tags: tags, TotalScans: rows[i].TotalScans}
	}
	totalPages := int(total) / limit
	if int(total)%limit > 0 {
		totalPages++
	}
	return &ListQRResult{QRCodes: out, Total: total, Page: page, TotalPages: totalPages}, nil
}

// Update edits the QR code. Title, tags and design live on the QR row; the
// destination and expiration go through LinkService.Update and routing rules
// through LinkService.SetRoutingRules. prevLink is the link before the update
// when its destination, expiration or routing rules may have changed (callers
// invalidate its cached redirect), otherwise nil. prevLink may be non-nil even
// when err != nil (a later step failed after the link changed); callers
// invalidate it.
func (s *QRServiceImpl) Update(ctx context.Context, userID, id uint64, input UpdateQRCodeInput) (*QRCodeDetails, *model.Link, error) {
	row, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return nil, nil, err
	}
	var rc model.JSONMap
	if input.RenderCustomizations != nil {
		if rc, err = normalizeRenderCustomizations(input.RenderCustomizations); err != nil {
			return nil, nil, err
		}
	}

	qr := row.QRCode
	link := row.ToLink()
	var prev *model.Link
	changed := []string{}

	dest := strings.TrimSpace(input.LongURL)
	if dest != "" || input.ExpiresAt != nil || input.ClearExpiration {
		updated, p, err := s.links.Update(ctx, userID, link.ID, UpdateLinkInput{
			OriginalURL:     dest,
			ExpiresAt:       input.ExpiresAt,
			ClearExpiration: input.ClearExpiration,
		})
		if err != nil {
			return nil, nil, err
		}
		if dest != "" && dest != link.OriginalURL {
			changed = append(changed, "long_url")
		}
		if input.ExpiresAt != nil || input.ClearExpiration {
			changed = append(changed, "expires_at")
		}
		link, prev = *updated, p
	}
	if input.RoutingRules != nil {
		if prev == nil {
			snapshot := link
			prev = &snapshot
		}
		if _, err := s.links.SetRoutingRules(ctx, userID, link.ID, *input.RoutingRules); err != nil {
			return nil, prev, err
		}
		changed = append(changed, "routing_rules")
	}

	rowChanged := false
	if input.Title != nil {
		qr.Title = qrTitle(*input.Title)
		rowChanged = true
		changed = append(changed, "title")
	}
	if rc != nil {
		qr.RenderCustomizations = rc
		rowChanged = true
		changed = append(changed, "render_customizations")
	}
	if rowChanged {
		if err := s.qrRepo.Update(ctx, &qr); err != nil {
			if errors.Is(err, repository.ErrQRCodeNotFound) {
				return nil, prev, ErrQRCodeNotFound
			}
			return nil, prev, err
		}
	}

	var tags []string
	if input.Tags != nil {
		tags = repository.NormalizeTags(*input.Tags)
		if err := s.qrRepo.SetTags(ctx, qr.ID, tags); err != nil {
			return nil, prev, err
		}
		changed = append(changed, "tags")
	} else if tags, err = s.tagsOf(ctx, qr.ID); err != nil {
		return nil, prev, err
	}

	s.activity.Log(detached(ctx), userID, qr.GroupID, ActionQRUpdate, EntityQRCode, qr.ID,
		map[string]any{"short_code": link.ShortCode, "fields": changed})
	return &QRCodeDetails{QRCode: qr, Link: link, Tags: tags, TotalScans: row.TotalScans}, prev, nil
}

// Delete removes the QR code. A QR-owned link (source 'qr') is deleted
// instead, which cascades to the QR row (linkDeleted); a Links-list link
// stays and only the QR row is deleted. Tags are not loaded.
func (s *QRServiceImpl) Delete(ctx context.Context, userID, id uint64) (*QRCodeDetails, bool, error) {
	row, err := s.getOwned(ctx, userID, id)
	if err != nil {
		return nil, false, err
	}
	d := &QRCodeDetails{QRCode: row.QRCode, Link: row.ToLink(), TotalScans: row.TotalScans}
	linkDeleted := d.Link.Source == model.LinkSourceQR
	if linkDeleted {
		// Deleting the link removes the QR row too (fk_qr_codes_link is
		// ON DELETE CASCADE), so a failure leaves both in place.
		if err := s.linkRepo.Delete(ctx, d.Link.ID); err != nil {
			if errors.Is(err, repository.ErrLinkNotFound) {
				return nil, false, ErrQRCodeNotFound
			}
			logger.Error(ctx, "qr-service: failed to delete qr-owned link",
				zap.Uint64("qr_code_id", row.ID),
				zap.Uint64("link_id", d.Link.ID),
				zap.Error(err),
			)
			return nil, false, err
		}
	} else if err := s.qrRepo.Delete(ctx, row.ID); err != nil {
		if errors.Is(err, repository.ErrQRCodeNotFound) {
			return nil, false, ErrQRCodeNotFound
		}
		return nil, false, err
	}
	s.activity.Log(detached(ctx), userID, row.GroupID, ActionQRDelete, EntityQRCode, row.ID,
		map[string]any{"short_code": d.Link.ShortCode, "link_deleted": linkDeleted})
	return d, linkDeleted, nil
}

// groupQRs loads the distinct ids and fails with ErrQRCodeNotFound unless
// every one exists and belongs to groupID.
func (s *QRServiceImpl) groupQRs(ctx context.Context, groupID uint64, ids []uint64) ([]uint64, []repository.QRCodeRow, error) {
	seen := make(map[uint64]bool, len(ids))
	unique := make([]uint64, 0, len(ids))
	for _, id := range ids {
		if !seen[id] {
			seen[id] = true
			unique = append(unique, id)
		}
	}
	if len(unique) == 0 {
		return nil, nil, ErrQRCodeNotFound
	}
	rows, err := s.qrRepo.GetByIDs(ctx, unique)
	if err != nil {
		return nil, nil, err
	}
	if len(rows) != len(unique) {
		return nil, nil, ErrQRCodeNotFound
	}
	for _, r := range rows {
		if r.GroupID != groupID {
			return nil, nil, ErrQRCodeNotFound
		}
	}
	return unique, rows, nil
}

// SetArchived hides or shows the QR codes (all must belong to groupID) and
// returns how many were changed. The links' archived flag is not touched.
func (s *QRServiceImpl) SetArchived(ctx context.Context, userID, groupID uint64, ids []uint64, archived bool) (int, error) {
	unique, rows, err := s.groupQRs(ctx, groupID, ids)
	if err != nil {
		return 0, err
	}
	if err := s.qrRepo.SetArchived(ctx, unique, archived); err != nil {
		return 0, err
	}
	action := ActionQRUnhide
	if archived {
		action = ActionQRHide
	}
	for i := range rows {
		s.activity.Log(detached(ctx), userID, groupID, action, EntityQRCode, rows[i].ID,
			map[string]any{"short_code": rows[i].LinkShortCode})
	}
	return len(unique), nil
}

// BulkTags adds and removes tags on QR codes that must all belong to groupID.
func (s *QRServiceImpl) BulkTags(ctx context.Context, groupID uint64, ids []uint64, add, remove []string) error {
	unique, _, err := s.groupQRs(ctx, groupID, ids)
	if err != nil {
		return err
	}
	if add = repository.NormalizeTags(add); len(add) > 0 {
		if err := s.qrRepo.AddTags(ctx, unique, add); err != nil {
			return err
		}
	}
	if remove = repository.NormalizeTags(remove); len(remove) > 0 {
		if err := s.qrRepo.RemoveTags(ctx, unique, remove); err != nil {
			return err
		}
	}
	return nil
}

// Export returns one CSV row per QR code matching f (columns per
// QRExportHeader, header not included).
func (s *QRServiceImpl) Export(ctx context.Context, groupID uint64, f repository.QRFilter) ([][]string, error) {
	rows := [][]string{}
	domains := map[uint64]string{}
	for offset := 0; offset < maxExportRows; offset += exportPageSize {
		page, _, err := s.qrRepo.ListByGroup(ctx, groupID, f, exportPageSize, offset)
		if err != nil {
			logger.Error(ctx, "qr-service: failed to export qr codes",
				zap.Uint64("group_id", groupID),
				zap.Error(err),
			)
			return nil, err
		}
		for i := range page {
			r := &page[i]
			link := r.ToLink()
			domainName := ""
			if link.DomainID != nil {
				name, ok := domains[*link.DomainID]
				if !ok {
					if d, err := s.domainRepo.GetByID(ctx, *link.DomainID); err == nil {
						name = d.Domain
					}
					domains[*link.DomainID] = name
				}
				domainName = name
			}
			title := ""
			if r.Title != nil {
				title = *r.Title
			} else if link.Title != nil {
				title = *link.Title
			}
			status := "Active"
			if r.Archived {
				status = "Hidden"
			}
			rows = append(rows, []string{
				csvSafe(title), csvSafe(ShortURL(s.baseURL, &link, domainName)), csvSafe(link.OriginalURL),
				csvSafe(r.CreatedAt.UTC().Format(time.RFC3339)), csvSafe(strconv.FormatInt(r.TotalScans, 10)), csvSafe(status),
			})
		}
		if len(page) < exportPageSize {
			break
		}
	}
	return rows, nil
}
