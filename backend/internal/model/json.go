package model

import (
	"database/sql/driver"
	"encoding/json"
	"fmt"
)

// StringList is a []string stored as a JSON array column.
// SQL NULL scans to an empty (non-nil) list; a nil list is stored as "[]".
type StringList []string

func (l *StringList) Scan(src any) error {
	b, err := jsonBytes(src)
	if err != nil {
		return fmt.Errorf("StringList: %w", err)
	}
	if b == nil {
		*l = StringList{}
		return nil
	}
	var out []string
	if err := json.Unmarshal(b, &out); err != nil {
		return fmt.Errorf("StringList: %w", err)
	}
	if out == nil {
		out = []string{}
	}
	*l = out
	return nil
}

func (l StringList) Value() (driver.Value, error) {
	if l == nil {
		return "[]", nil
	}
	b, err := json.Marshal([]string(l))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func (l StringList) MarshalJSON() ([]byte, error) {
	if l == nil {
		return []byte("[]"), nil
	}
	return json.Marshal([]string(l))
}

// JSONMap is a JSON object column. SQL NULL scans to an empty map; a nil map
// is stored as SQL NULL.
type JSONMap map[string]any

func (m *JSONMap) Scan(src any) error {
	b, err := jsonBytes(src)
	if err != nil {
		return fmt.Errorf("JSONMap: %w", err)
	}
	if b == nil {
		*m = JSONMap{}
		return nil
	}
	out := map[string]any{}
	if err := json.Unmarshal(b, &out); err != nil {
		return fmt.Errorf("JSONMap: %w", err)
	}
	if out == nil {
		out = map[string]any{}
	}
	*m = out
	return nil
}

func (m JSONMap) Value() (driver.Value, error) {
	if m == nil {
		return nil, nil
	}
	b, err := json.Marshal(map[string]any(m))
	if err != nil {
		return nil, err
	}
	return string(b), nil
}

func jsonBytes(src any) ([]byte, error) {
	switch v := src.(type) {
	case nil:
		return nil, nil
	case []byte:
		return v, nil
	case string:
		return []byte(v), nil
	default:
		return nil, fmt.Errorf("unsupported source type %T", src)
	}
}
