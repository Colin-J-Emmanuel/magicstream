package llm

import (
	"errors"
	"testing"
)

func TestParseRanking(t *testing.T) {
	tests := []struct {
		raw       string
		wantName  string
		wantValue int
		wantErr   bool
	}{
		{"Excellent", "Excellent", 1, false},
		{"  good\n", "Good", 2, false},
		{"Okay.", "Okay", 3, false},
		{"BAD", "Bad", 4, false},
		{"**Terrible**", "Terrible", 5, false},
		{"Not Bad", "", 0, true},                        // contains "Bad", means the opposite
		{"Excellent, though leaning Good", "", 0, true}, // two answers
		{"Amazing", "", 0, true},                        // not on the scale
		{"", "", 0, true},                               // empty reply
	}

	for _, tt := range tests {
		t.Run(tt.raw, func(t *testing.T) {
			got, err := ParseRanking(tt.raw)
			if tt.wantErr {
				if !errors.Is(err, ErrInvalidRanking) {
					t.Fatalf("want ErrInvalidRanking, got err=%v ranking=%+v", err, got)
				}
				return
			}
			if err != nil {
				t.Fatalf("unexpected error: %v", err)
			}
			if got.RankingName != tt.wantName || got.RankingValue != tt.wantValue {
				t.Fatalf("got %+v, want %s (%d)", got, tt.wantName, tt.wantValue)
			}
		})
	}
}
