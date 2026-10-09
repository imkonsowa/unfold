package golangci

import (
	"testing"
)

func TestNew(t *testing.T) {
	t.Parallel()

	tests := []struct {
		name     string
		settings any
		wantErr  bool
	}{
		{
			name:     "no settings",
			settings: nil,
		},
		{
			name:     "empty settings",
			settings: map[string]any{},
		},
		{
			name: "settings belong in .unfold.yml",
			settings: map[string]any{
				"min-elements": 2,
			},
			wantErr: true,
		},
	}
	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			t.Parallel()

			p, err := New(tt.settings)
			if tt.wantErr {
				if err == nil {
					t.Error("New = nil error, want one")
				}

				return
			}

			if err != nil {
				t.Fatal(err)
			}

			analyzers, err := p.BuildAnalyzers()
			if err != nil || len(analyzers) != 1 || analyzers[0].Name != "unfold" {
				t.Errorf("BuildAnalyzers = %v, %v; want the unfold analyzer", analyzers, err)
			}
		})
	}
}
