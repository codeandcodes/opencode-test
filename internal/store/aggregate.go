package store

import "sort"

// CellAgg summarizes all completed samples of one (task, model) cell.
type CellAgg struct {
	Samples     int     `json:"samples"`
	Passes      int     `json:"passes"`
	Fails       int     `json:"fails"`
	Dones       int     `json:"dones"`
	VerdictGood int     `json:"verdict_good"` // legacy binary verdicts
	VerdictBad  int     `json:"verdict_bad"`
	RatingCount int     `json:"rating_count"`
	RatingAvg   float64 `json:"rating_avg"`
	MedianTps   float64 `json:"median_tps"`
}

// Aggregate summarizes a cell's history. Only completed measurements
// (done/pass/fail) count as samples; infrastructure failures are ignored.
func Aggregate(history []Result) CellAgg {
	var agg CellAgg
	var tps []float64
	for _, r := range history {
		switch r.Status {
		case "pass":
			agg.Passes++
		case "fail":
			agg.Fails++
		case "done":
			agg.Dones++
		default:
			continue
		}
		agg.Samples++
		if r.Verdict != nil {
			if r.Verdict.Rating > 0 {
				agg.RatingCount++
				agg.RatingAvg += float64(r.Verdict.Rating)
			} else {
				switch r.Verdict.Verdict {
				case "good":
					agg.VerdictGood++
				case "bad":
					agg.VerdictBad++
				}
			}
		}
		out := r.TokensOut + r.TokensReasoning
		if r.GenSeconds > 0 && out > 0 {
			tps = append(tps, float64(out)/r.GenSeconds)
		}
	}
	if agg.RatingCount > 0 {
		agg.RatingAvg /= float64(agg.RatingCount)
	}
	if len(tps) > 0 {
		sort.Float64s(tps)
		mid := len(tps) / 2
		if len(tps)%2 == 1 {
			agg.MedianTps = tps[mid]
		} else {
			agg.MedianTps = (tps[mid-1] + tps[mid]) / 2
		}
	}
	return agg
}
