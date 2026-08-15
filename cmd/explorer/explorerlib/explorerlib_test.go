package explorerlib

import (
	"testing"
	"time"
)

func TestFormatBlockAge(t *testing.T) {
	now := time.UnixMilli(1_700_000_000_000).UTC()

	tests := []struct {
		name        string
		timestampMs uint64
		want        string
	}{
		{
			name:        "seconds",
			timestampMs: uint64(now.Add(-13 * time.Second).UnixMilli()),
			want:        "13s ago",
		},
		{
			name:        "minutes",
			timestampMs: uint64(now.Add(-(time.Minute + 14*time.Second)).UnixMilli()),
			want:        "1m 14s ago",
		},
		{
			name:        "hours",
			timestampMs: uint64(now.Add(-(2*time.Hour + 3*time.Minute)).UnixMilli()),
			want:        "2h 03m ago",
		},
		{
			name:        "days",
			timestampMs: uint64(now.Add(-(3*24*time.Hour + 2*time.Hour)).UnixMilli()),
			want:        "3d 2h ago",
		},
		{
			name:        "future",
			timestampMs: uint64(now.Add(time.Second).UnixMilli()),
			want:        "0s ago",
		},
		{
			name:        "missing timestamp",
			timestampMs: 0,
			want:        "unknown",
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			if got := formatBlockAge(tt.timestampMs, now); got != tt.want {
				t.Fatalf("formatBlockAge() = %q, want %q", got, tt.want)
			}
		})
	}
}
