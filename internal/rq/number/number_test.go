package number

import (
	"encoding/json"
	"testing"
)

func TestToFloat64(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		input any
		ok    bool
		want  float64
	}{
		{name: "int", input: int(10), ok: true, want: 10},
		{name: "float64", input: 12.5, ok: true, want: 12.5},
		{name: "json_number", input: json.Number("42"), ok: true, want: 42},
		{name: "non_numeric", input: "x", ok: false, want: 0},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, ok := ToFloat64(tt.input)
			if ok != tt.ok {
				t.Fatalf("ToFloat64(%v) ok = %v, want %v", tt.input, ok, tt.ok)
			}
			if got != tt.want {
				t.Fatalf("ToFloat64(%v) value = %v, want %v", tt.input, got, tt.want)
			}
		})
	}
}

func TestToStrictInt(t *testing.T) {
	t.Parallel()

	if got, err := ToStrictInt(int64(7)); err != nil || got != 7 {
		t.Fatalf("ToStrictInt(int64(7)) = (%d, %v), want (7, nil)", got, err)
	}

	if _, err := ToStrictInt(4.2); err == nil {
		t.Fatal("ToStrictInt(4.2) expected error")
	}
}
