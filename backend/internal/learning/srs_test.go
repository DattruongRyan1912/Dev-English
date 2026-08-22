package learning

import (
	"testing"
	"time"
)

func TestNextReviewFollowsProductSchedule(t *testing.T) {
	now := time.Date(2026, 8, 22, 0, 0, 0, 0, time.UTC)
	cases := []struct {
		mastery float64
		want    time.Duration
	}{
		{0.0, 24 * time.Hour},
		{0.2, 3 * 24 * time.Hour},
		{0.45, 7 * 24 * time.Hour},
		{0.7, 14 * 24 * time.Hour},
		{0.95, 30 * 24 * time.Hour},
	}
	for _, tc := range cases {
		got := NextReview(now, tc.mastery, true).Sub(now)
		if got != tc.want {
			t.Fatalf("mastery %.2f: got %s, want %s", tc.mastery, got, tc.want)
		}
	}
}

func TestUpdatedMasteryIsBounded(t *testing.T) {
	if got := UpdatedMastery(0.99, true, 100); got != 1 {
		t.Fatalf("expected mastery to cap at 1, got %.2f", got)
	}
	if got := UpdatedMastery(0.01, false, 0); got != 0 {
		t.Fatalf("expected mastery to floor at 0, got %.2f", got)
	}
}
