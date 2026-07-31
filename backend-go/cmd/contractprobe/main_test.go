package main

import (
	"reflect"
	"testing"
)

func TestNormalizeOnlyAllowsDeclaredNondeterministicFields(t *testing.T) {
	input := map[string]any{
		"id":              "stable-track-id",
		"publicId":        "random-public-id",
		"serverTimestamp": jsonNumber("1234"),
		"items":           []any{"second", "first"},
	}
	want := map[string]any{
		"id":              "stable-track-id",
		"publicId":        "<public-id>",
		"serverTimestamp": "<timestamp>",
		"items":           []any{"second", "first"},
	}
	if got := normalize(input, ""); !reflect.DeepEqual(got, want) {
		t.Fatalf("normalize() = %#v, want %#v", got, want)
	}
}

// jsonNumber mirrors the concrete value used by decodeAndNormalize without
// making the test depend on floating-point conversion.
type jsonNumber string
