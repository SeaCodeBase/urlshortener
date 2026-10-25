package model

import "time"

// Bulk upload status values (bulk_uploads.status).
const (
	BulkStatusQueued     = "queued"
	BulkStatusProcessing = "processing"
	BulkStatusDone       = "done"
	BulkStatusFailed     = "failed"
)

// BulkUpload is a CSV link import processed by the bulk worker. SourceCSV is
// the uploaded file and ResultCSV the per-row outcome (nil until finished);
// neither is serialized, and list queries leave both empty.
type BulkUpload struct {
	ID        uint64    `db:"id" json:"id"`
	UserID    uint64    `db:"user_id" json:"user_id"`
	GroupID   uint64    `db:"group_id" json:"group_id"`
	Filename  string    `db:"filename" json:"filename"`
	Status    string    `db:"status" json:"status"`
	Total     int       `db:"total" json:"total"`
	Processed int       `db:"processed" json:"processed"`
	Succeeded int       `db:"succeeded" json:"succeeded"`
	Failed    int       `db:"failed" json:"failed"`
	SourceCSV string    `db:"source_csv" json:"-"`
	ResultCSV *string   `db:"result_csv" json:"-"`
	Error     string    `db:"error" json:"error"`
	CreatedAt time.Time `db:"created_at" json:"created_at"`
	UpdatedAt time.Time `db:"updated_at" json:"updated_at"`
}
