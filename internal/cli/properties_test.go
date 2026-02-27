package cli

import (
	"reflect"
	"strings"
	"testing"
)

func TestParsePropertyMode(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		want    PropertyMode
		wantErr bool
	}{
		{name: "off", input: "off", want: PropertyModeOff},
		{name: "warn", input: "warn", want: PropertyModeWarn},
		{name: "strict", input: "strict", want: PropertyModeStrict},
		{name: "trim and case", input: " WARN ", want: PropertyModeWarn},
		{name: "invalid", input: "nope", wantErr: true},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := ParsePropertyMode(tt.input)
			if tt.wantErr {
				if err == nil {
					t.Fatalf("expected error, got nil")
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got != tt.want {
				t.Fatalf("got %q, want %q", got, tt.want)
			}
		})
	}
}

func TestParseFrontmatterProperties(t *testing.T) {
	content := strings.Join([]string{
		"---",
		"notion-id: abc123",
		"notion:",
		"  source: test",
		"Status: Todo",
		"Done: true",
		"Count: 3",
		"props:",
		"  Priority: High",
		"  Tags:",
		"    - cli",
		"    - notion",
		"---",
		"",
		"# Title",
	}, "\n")

	got, err := ParseFrontmatterProperties(content)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}

	want := map[string]any{
		"Status":   "Todo",
		"Done":     true,
		"Count":    3,
		"Priority": "High",
		"Tags":     []any{"cli", "notion"},
	}
	if !reflect.DeepEqual(got, want) {
		t.Fatalf("got %#v, want %#v", got, want)
	}
}

func TestParseFrontmatterPropertiesInvalidYAML(t *testing.T) {
	content := "---\nfoo: [\n---\nbody\n"
	_, err := ParseFrontmatterProperties(content)
	if err == nil {
		t.Fatalf("expected error for invalid YAML")
	}
}

func TestParsePropertiesFlags(t *testing.T) {
	props, errs := ParsePropertiesFlags(
		[]string{"Status=Todo;Priority=Low;Done=false;Count=2"},
		[]string{"Priority=High", "Done=true"},
	)
	if len(errs) != 0 {
		t.Fatalf("unexpected errors: %v", errs)
	}

	want := map[string]any{
		"Status":   "Todo",
		"Priority": "High",
		"Done":     true,
		"Count":    int64(2),
	}
	if !reflect.DeepEqual(props, want) {
		t.Fatalf("got %#v, want %#v", props, want)
	}
}

func TestParsePropertiesFlagsCollectsErrors(t *testing.T) {
	_, errs := ParsePropertiesFlags(
		[]string{"Status=Todo;broken"},
		[]string{"=missing", "ok=true"},
	)
	if len(errs) != 2 {
		t.Fatalf("expected 2 errors, got %d (%v)", len(errs), errs)
	}
}
