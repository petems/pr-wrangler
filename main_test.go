package main

import "testing"

func TestParseTUIOptions(t *testing.T) {
	tests := []struct {
		name        string
		args        []string
		wantNoCache bool
		wantErr     bool
	}{
		{
			name: "empty",
		},
		{
			name:        "no cache",
			args:        []string{"--no-cache"},
			wantNoCache: true,
		},
		{
			name:    "unknown flag",
			args:    []string{"--wat"},
			wantErr: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got, err := parseTUIOptions(tt.args)
			if tt.wantErr {
				if err == nil {
					t.Fatal("expected error")
				}
				return
			}
			if err != nil {
				t.Fatalf("parseTUIOptions: %v", err)
			}
			if got.noCache != tt.wantNoCache {
				t.Fatalf("noCache = %v, want %v", got.noCache, tt.wantNoCache)
			}
		})
	}
}
