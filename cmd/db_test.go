package cmd

import "testing"

func TestExtractViewID(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name  string
		in    string
		want  string
		found bool
	}{
		{
			name:  "view scheme",
			in:    "view://e0e4fd04-a7e6-4b04-a571-956fd21b3862",
			want:  "e0e4fd04-a7e6-4b04-a571-956fd21b3862",
			found: true,
		},
		{
			name:  "database url query",
			in:    "https://www.notion.so/20012a4c46544443916732b7ea1f6641?v=e0e4fd04a7e64b04a571956fd21b3862",
			want:  "e0e4fd04-a7e6-4b04-a571-956fd21b3862",
			found: true,
		},
		{
			name:  "uuid",
			in:    "e0e4fd04a7e64b04a571956fd21b3862",
			want:  "e0e4fd04-a7e6-4b04-a571-956fd21b3862",
			found: true,
		},
		{
			name:  "name",
			in:    "My View",
			want:  "",
			found: false,
		},
	}

	for _, tt := range tests {
		tt := tt
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()
			got, ok := extractViewID(tt.in)
			if ok != tt.found {
				t.Fatalf("found mismatch: got %v want %v", ok, tt.found)
			}
			if got != tt.want {
				t.Fatalf("id mismatch: got %q want %q", got, tt.want)
			}
		})
	}
}

func TestBuildViewOptionsConflicts(t *testing.T) {
	t.Parallel()

	_, err := buildViewOptions(viewOptionInputs{
		ShowPageIcon:   true,
		NoShowPageIcon: true,
	})
	if err == nil {
		t.Fatal("expected conflict error")
	}
}
