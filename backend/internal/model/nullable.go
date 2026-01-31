package model

import (
	"database/sql"
	"encoding/json"
	"time"
)

// NullTime is a wrapper around sql.NullTime that serializes to JSON properly.
// It serializes to null when invalid, or to an ISO8601 string when valid.
type NullTime struct {
	sql.NullTime
}

func (nt NullTime) MarshalJSON() ([]byte, error) {
	if !nt.Valid {
		return []byte("null"), nil
	}
	return json.Marshal(nt.Time.Format(time.RFC3339))
}

func (nt *NullTime) UnmarshalJSON(data []byte) error {
	if string(data) == "null" || string(data) == `""` {
		nt.Valid = false
		return nil
	}
	var s string
	if err := json.Unmarshal(data, &s); err != nil {
		return err
	}
	t, err := time.Parse(time.RFC3339, s)
	if err != nil {
		return err
	}
	nt.Time = t
	nt.Valid = true
	return nil
}
