package model

import (
	"encoding/json"
	"reflect"
	"testing"
)

func TestStringList_ScanValue(t *testing.T) {
	in := StringList{"a", "b"}
	v, err := in.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	if s, ok := v.(string); !ok || s != `["a","b"]` {
		t.Fatalf("Value = %#v, want string [\"a\",\"b\"]", v)
	}

	for _, src := range []any{v, []byte(v.(string))} {
		var out StringList
		if err := out.Scan(src); err != nil {
			t.Fatalf("Scan(%T): %v", src, err)
		}
		if !reflect.DeepEqual(out, in) {
			t.Fatalf("Scan(%T) = %#v, want %#v", src, out, in)
		}
	}

	var null StringList
	if err := null.Scan(nil); err != nil {
		t.Fatalf("Scan(nil): %v", err)
	}
	if null == nil || len(null) != 0 {
		t.Fatalf("Scan(nil) = %#v, want empty non-nil slice", null)
	}

	var empty StringList
	v, err = empty.Value()
	if err != nil || v != "[]" {
		t.Fatalf("nil StringList Value = %#v, %v; want \"[]\"", v, err)
	}
	b, _ := json.Marshal(empty)
	if string(b) != "[]" {
		t.Fatalf("nil StringList JSON = %s, want []", b)
	}

	var bad StringList
	if err := bad.Scan(42); err == nil {
		t.Fatal("Scan(int) should fail")
	}
}

func TestJSONMap_ScanValue(t *testing.T) {
	in := JSONMap{"k": "v", "n": float64(1)}
	v, err := in.Value()
	if err != nil {
		t.Fatalf("Value: %v", err)
	}
	var out JSONMap
	if err := out.Scan([]byte(v.(string))); err != nil {
		t.Fatalf("Scan: %v", err)
	}
	if !reflect.DeepEqual(out, in) {
		t.Fatalf("round trip = %#v, want %#v", out, in)
	}

	var null JSONMap
	if err := null.Scan(nil); err != nil || null == nil || len(null) != 0 {
		t.Fatalf("Scan(nil) = %#v, %v; want empty map", null, err)
	}
	var nilMap JSONMap
	if v, err := nilMap.Value(); err != nil || v != nil {
		t.Fatalf("nil JSONMap Value = %#v, %v; want SQL NULL", v, err)
	}
}
