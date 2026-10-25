package service

import (
	"bytes"
	"context"
	"encoding/csv"
	"errors"
	"io"
	"path"
	"strconv"
	"strings"
	"unicode"
	"unicode/utf8"

	"github.com/SeaCodeBase/urlshortener/internal/model"
	"github.com/SeaCodeBase/urlshortener/internal/repository"
)

// Compile-time check: BulkServiceImpl implements BulkService
var _ BulkService = (*BulkServiceImpl)(nil)

const (
	// BulkMaxBytes is the largest accepted CSV upload (2 MiB).
	BulkMaxBytes = 2 << 20
	// BulkMaxRows is the most data rows one upload may contain.
	BulkMaxRows = 1000

	bulkFilenameMax     = 255
	bulkDefaultFilename = "upload.csv"
)

// ErrBulkNotFinished: the upload's results are not available yet (409).
var ErrBulkNotFinished = errors.New("upload not finished")

// BulkHeader is the required CSV header, in order.
var BulkHeader = []string{"long_url", "title", "custom_back_half", "tags", "domain"}

// BulkResultHeader is the header of the per-row results CSV.
var BulkResultHeader = []string{"row", "long_url", "short_url", "status", "error"}

// Bulk result row statuses.
const (
	BulkRowOK    = "ok"
	BulkRowError = "error"
)

// BulkRow is one data row of an upload. Line is its 1-based number among the
// file's non-blank data rows (the header is not counted).
type BulkRow struct {
	Line           int
	LongURL        string
	Title          string
	CustomBackHalf string
	Domain         string
	Tags           []string
}

// BulkResultRow is one line of the results CSV.
type BulkResultRow struct {
	Row      int
	LongURL  string
	ShortURL string
	Status   string
	Error    string
}

type BulkServiceDeps struct {
	Bulk     repository.BulkUploadRepository
	Activity ActivityService
}

type BulkServiceImpl struct {
	bulk     repository.BulkUploadRepository
	activity ActivityService
}

func NewBulkService(d BulkServiceDeps) *BulkServiceImpl {
	return &BulkServiceImpl{bulk: d.Bulk, activity: d.Activity}
}

// ParseBulkCSV reads an upload: the header must be BulkHeader (compared
// case-insensitively after trimming; a UTF-8 BOM and trailing empty header
// cells are ignored), cells are trimmed, short rows are padded, fully blank
// rows are skipped and tags are split on "|". Quoting is strict RFC 4180, so
// an unterminated or stray quote rejects the file instead of silently
// merging rows. A structurally broken file, a
// row with more than five non-empty columns, no data rows or more than
// BulkMaxRows rows is a *ValidationError. Semantic checks (URL, domain,
// back-half) are left to link creation.
func ParseBulkCSV(r io.Reader) ([]BulkRow, error) {
	cr := csv.NewReader(r)
	cr.FieldsPerRecord = -1
	cr.ReuseRecord = true

	header, err := cr.Read()
	if errors.Is(err, io.EOF) {
		return nil, invalid("the file has no rows")
	}
	if err != nil {
		return nil, csvParseError(err)
	}
	if len(header) > 0 {
		header[0] = strings.TrimPrefix(header[0], "\uFEFF")
	}
	header = trimTrailingEmpty(header)
	if len(header) != len(BulkHeader) {
		return nil, invalid("the header must be %s", strings.Join(BulkHeader, ","))
	}
	for i, h := range header {
		if !strings.EqualFold(strings.TrimSpace(h), BulkHeader[i]) {
			return nil, invalid("the header must be %s", strings.Join(BulkHeader, ","))
		}
	}

	rows := []BulkRow{}
	for {
		rec, err := cr.Read()
		if errors.Is(err, io.EOF) {
			break
		}
		if err != nil {
			return nil, csvParseError(err)
		}
		fields := make([]string, len(rec))
		for i, v := range rec {
			fields[i] = strings.TrimSpace(v)
		}
		fields = trimTrailingEmpty(fields)
		if len(fields) == 0 {
			continue
		}
		if len(fields) > len(BulkHeader) {
			line, _ := cr.FieldPos(0)
			return nil, invalid("line %d: expected at most %d columns", line, len(BulkHeader))
		}
		if len(rows) == BulkMaxRows {
			return nil, invalid("a file can have at most %d rows", BulkMaxRows)
		}
		for len(fields) < len(BulkHeader) {
			fields = append(fields, "")
		}
		rows = append(rows, BulkRow{
			Line:           len(rows) + 1,
			LongURL:        fields[0],
			Title:          fields[1],
			CustomBackHalf: fields[2],
			Tags:           splitBulkTags(fields[3]),
			Domain:         fields[4],
		})
	}
	if len(rows) == 0 {
		return nil, invalid("the file has no rows")
	}
	return rows, nil
}

func csvParseError(err error) error {
	var pe *csv.ParseError
	if errors.As(err, &pe) {
		return invalid("invalid CSV on line %d: %v", pe.Line, pe.Err)
	}
	return invalid("the file is not a valid CSV")
}

func trimTrailingEmpty(fields []string) []string {
	n := len(fields)
	for n > 0 && strings.TrimSpace(fields[n-1]) == "" {
		n--
	}
	return fields[:n]
}

func splitBulkTags(v string) []string {
	tags := []string{}
	for _, t := range strings.Split(v, "|") {
		if t = strings.TrimSpace(t); t != "" {
			tags = append(tags, t)
		}
	}
	return tags
}

// SanitizeBulkFilename keeps the base name of a client-supplied filename
// (either path separator), drops control characters and invalid UTF-8 and
// cuts it to 255 characters; an empty result becomes "upload.csv".
func SanitizeBulkFilename(name string) string {
	name = strings.ToValidUTF8(name, "")
	name = strings.Map(func(r rune) rune {
		if unicode.IsControl(r) {
			return -1
		}
		return r
	}, name)
	name = strings.ReplaceAll(name, `\`, "/")
	name = strings.TrimSpace(path.Base(name))
	if name == "" || name == "." || name == "/" || name == ".." {
		return bulkDefaultFilename
	}
	if r := []rune(name); len(r) > bulkFilenameMax {
		name = string(r[:bulkFilenameMax])
	}
	return name
}

// EncodeBulkResults renders the results CSV (BulkResultHeader first); every
// cell goes through csvSafe.
func EncodeBulkResults(rows []BulkResultRow) (string, error) {
	return encodeBulkResults(rows, true)
}

// EncodeBulkResultRows encodes rows like EncodeBulkResults but without the
// header, to append to a results CSV stored earlier.
func EncodeBulkResultRows(rows []BulkResultRow) (string, error) {
	return encodeBulkResults(rows, false)
}

func encodeBulkResults(rows []BulkResultRow, header bool) (string, error) {
	var buf bytes.Buffer
	w := csv.NewWriter(&buf)
	if header {
		if err := w.Write(BulkResultHeader); err != nil {
			return "", err
		}
	}
	for _, r := range rows {
		rec := []string{strconv.Itoa(r.Row), r.LongURL, r.ShortURL, r.Status, r.Error}
		for i := range rec {
			rec[i] = csvSafe(rec[i])
		}
		if err := w.Write(rec); err != nil {
			return "", err
		}
	}
	w.Flush()
	if err := w.Error(); err != nil {
		return "", err
	}
	return buf.String(), nil
}

func (s *BulkServiceImpl) Create(ctx context.Context, userID, groupID uint64, filename string, csvBytes []byte) (*model.BulkUpload, error) {
	if len(csvBytes) > BulkMaxBytes {
		return nil, invalid("file is larger than 2 MB")
	}
	// source_csv is utf8mb4 in strict mode: anything else fails the INSERT.
	if !utf8.Valid(csvBytes) {
		return nil, invalid("the file must be UTF-8 encoded (in Excel choose \"CSV UTF-8\")")
	}
	rows, err := ParseBulkCSV(bytes.NewReader(csvBytes))
	if err != nil {
		return nil, err
	}
	b := &model.BulkUpload{
		UserID:    userID,
		GroupID:   groupID,
		Filename:  SanitizeBulkFilename(filename),
		Status:    model.BulkStatusQueued,
		Total:     len(rows),
		SourceCSV: string(csvBytes),
	}
	if err := s.bulk.Create(ctx, b); err != nil {
		return nil, err
	}
	s.activity.Log(ctx, userID, groupID, ActionBulkCreate, EntityBulk, b.ID,
		map[string]any{"filename": b.Filename, "total": b.Total})
	return b, nil
}

func (s *BulkServiceImpl) List(ctx context.Context, groupID uint64) ([]model.BulkUpload, error) {
	return s.bulk.ListByGroup(ctx, groupID)
}

func (s *BulkServiceImpl) Get(ctx context.Context, groupID, id uint64) (*model.BulkUpload, error) {
	b, err := s.bulk.GetByID(ctx, id)
	if err != nil {
		return nil, err
	}
	if b.GroupID != groupID {
		return nil, repository.ErrBulkUploadNotFound
	}
	return b, nil
}

func (s *BulkServiceImpl) Results(ctx context.Context, groupID, id uint64) (string, error) {
	b, err := s.Get(ctx, groupID, id)
	if err != nil {
		return "", err
	}
	if b.Status != model.BulkStatusDone && b.Status != model.BulkStatusFailed {
		return "", ErrBulkNotFinished
	}
	if b.ResultCSV == nil {
		return EncodeBulkResults(nil)
	}
	return *b.ResultCSV, nil
}
