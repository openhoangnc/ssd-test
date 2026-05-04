package bench

import "sort"

// CacheEstimate describes an SLC/DRAM cache cliff inferred from per-second
// samples. Detected is false when no clear cliff is observed (uniform speed,
// too few samples, or test ended before the cache was exhausted).
type CacheEstimate struct {
	Detected    bool
	Bytes       int64   // approximate cache size: bytes written before the cliff
	BurstSpeed  float64 // representative in-cache speed (bytes/s)
	SteadySpeed float64 // representative post-cache speed (bytes/s)
}

// EstimateCache walks the samples looking for a sustained drop from a fast
// "burst" plateau to a slower "steady" plateau. The bytes written before that
// drop approximates the SSD's fast write cache.
//
// Heuristic:
//   - steady = median speed of the last third of samples
//   - burst  = median speed of the first quarter of samples
//   - require burst >= 2 * steady (otherwise no real cliff)
//   - threshold = (burst + steady) / 2; first run of 3 consecutive samples at
//     or below the threshold marks the cliff. The cache size is the bytes
//     written by the sample just before that run.
func EstimateCache(samples []Sample) CacheEstimate {
	if len(samples) < 8 {
		return CacheEstimate{}
	}

	speeds := make([]float64, len(samples))
	for i, s := range samples {
		speeds[i] = s.BlockSpeed
	}

	tailStart := len(samples) * 2 / 3
	steady := median(speeds[tailStart:])

	headEnd := len(samples) / 4
	if headEnd < 2 {
		headEnd = 2
	}
	burst := median(speeds[:headEnd])

	if steady <= 0 || burst < steady*2 {
		return CacheEstimate{}
	}

	threshold := (burst + steady) / 2
	const window = 3
	for i := 0; i+window <= len(samples); i++ {
		below := true
		for j := 0; j < window; j++ {
			if samples[i+j].BlockSpeed > threshold {
				below = false
				break
			}
		}
		if !below {
			continue
		}
		if i == 0 {
			return CacheEstimate{}
		}
		return CacheEstimate{
			Detected:    true,
			Bytes:       samples[i-1].BytesWritten,
			BurstSpeed:  burst,
			SteadySpeed: steady,
		}
	}
	return CacheEstimate{}
}

func median(xs []float64) float64 {
	if len(xs) == 0 {
		return 0
	}
	cp := append([]float64(nil), xs...)
	sort.Float64s(cp)
	n := len(cp)
	if n%2 == 1 {
		return cp[n/2]
	}
	return (cp[n/2-1] + cp[n/2]) / 2
}
