package bench

import (
	"testing"
	"time"
)

func makeSamples(speeds []float64) []Sample {
	out := make([]Sample, len(speeds))
	var total int64
	for i, sp := range speeds {
		total += int64(sp)
		out[i] = Sample{
			T:            time.Unix(int64(i), 0),
			Elapsed:      time.Duration(i+1) * time.Second,
			BytesWritten: total,
			BlockBytes:   int64(sp),
			BlockSpeed:   sp,
		}
	}
	return out
}

func TestEstimateCache_ClearCliff(t *testing.T) {
	const G = 1 << 30
	const M = 1 << 20
	speeds := make([]float64, 0, 90)
	for i := 0; i < 10; i++ {
		s := 1.5 * G
		if i == 5 {
			s = 0.9 * G // brief dip should not trigger
		}
		speeds = append(speeds, s)
	}
	for i := 0; i < 80; i++ {
		s := 100.0 * M
		if i%7 == 0 {
			s = 200.0 * M
		}
		speeds = append(speeds, s)
	}
	got := EstimateCache(makeSamples(speeds))
	if !got.Detected {
		t.Fatalf("expected cache cliff to be detected")
	}
	wantLow := int64(10 * G)
	wantHigh := int64(16 * G)
	if got.Bytes < wantLow || got.Bytes > wantHigh {
		t.Errorf("cache size %d out of expected range [%d,%d]", got.Bytes, wantLow, wantHigh)
	}
}

func TestEstimateCache_NoCliff(t *testing.T) {
	const M = 1 << 20
	speeds := make([]float64, 60)
	for i := range speeds {
		speeds[i] = 500 * M
		if i%5 == 0 {
			speeds[i] = 480 * M
		}
	}
	if got := EstimateCache(makeSamples(speeds)); got.Detected {
		t.Errorf("did not expect cliff on uniform speeds, got %+v", got)
	}
}

func TestEstimateCache_TooFewSamples(t *testing.T) {
	if got := EstimateCache(makeSamples([]float64{1e9, 1e9, 1e9})); got.Detected {
		t.Errorf("did not expect detection with too few samples, got %+v", got)
	}
}
