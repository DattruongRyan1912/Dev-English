package learning

import "time"

var reviewIntervals = [...]time.Duration{
	24 * time.Hour,
	3 * 24 * time.Hour,
	7 * 24 * time.Hour,
	14 * 24 * time.Hour,
	30 * 24 * time.Hour,
}

// NextReview applies the V1 schedule from the product spec: 1d -> 3d -> 7d -> 14d -> 30d.
// A failed review moves the item back one step; a successful review advances it.
func NextReview(now time.Time, mastery float64, success bool) time.Time {
	index := int(mastery * float64(len(reviewIntervals)))
	if index < 0 {
		index = 0
	}
	if index >= len(reviewIntervals) {
		index = len(reviewIntervals) - 1
	}
	if !success && index > 0 {
		index--
	}
	return now.Add(reviewIntervals[index])
}

func UpdatedMastery(mastery float64, success bool, score float64) float64 {
	if success {
		mastery += 0.12 + score/1000
	}
	if !success {
		mastery -= 0.08
	}
	if mastery < 0 {
		return 0
	}
	if mastery > 1 {
		return 1
	}
	return mastery
}
