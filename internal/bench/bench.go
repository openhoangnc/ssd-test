// Package bench runs the sustained-write benchmark and emits one Sample per
// second on a channel. Display, export, and signal handling live elsewhere.
package bench

import (
	"context"
	"crypto/rand"
	"fmt"
	"os"
	"path/filepath"
	"strings"
	"time"
)

const (
	BlockSize  = 1 << 20 // 1 MiB write chunk
	SampleRate = time.Second
)

type Options struct {
	Dir      string // directory to write the test file in
	FileSize int64  // total bytes to write
}

// Sample is one observation emitted ~once per second.
type Sample struct {
	T            time.Time     // wall-clock time of the sample
	Elapsed      time.Duration // since the start of the run
	BytesWritten int64         // cumulative
	BlockBytes   int64         // bytes since previous sample
	BlockSpeed   float64       // bytes per second over this sample window
	AvgSpeed     float64       // bytes per second since start
}

// Result is the post-run summary the consumer assembles from observed samples.
type Result struct {
	Started   time.Time
	Finished  time.Time
	Dir       string
	FileSize  int64
	Written   int64
	Samples   []Sample
	Min, Max  float64 // bytes/s across block samples
	Avg       float64 // bytes/s overall
	Duration  time.Duration
	Cancelled bool // true if ctx was cancelled before completion
}

// Run executes the benchmark and emits Samples on the returned channel. The
// channel is closed when the run finishes (completion, cancellation, or error).
// The test file is created and removed by Run; the caller does not see it.
func Run(ctx context.Context, opts Options) (<-chan Sample, <-chan Result, error) {
	if opts.FileSize <= 0 {
		return nil, nil, fmt.Errorf("bench: FileSize must be > 0")
	}
	if opts.Dir == "" {
		opts.Dir = "."
	}

	suffix := make([]byte, 8)
	if _, err := rand.Read(suffix); err != nil {
		return nil, nil, fmt.Errorf("bench: rand.Read: %w", err)
	}
	path := filepath.Join(opts.Dir, fmt.Sprintf("ssd-test-%x.tmp", suffix))

	f, err := os.Create(path)
	if err != nil {
		return nil, nil, fmt.Errorf("bench: create %s: %w", path, err)
	}
	if err := f.Truncate(opts.FileSize); err != nil {
		f.Close()
		os.Remove(path)
		return nil, nil, fmt.Errorf("bench: pre-allocate %s: %w", path, err)
	}

	buf := make([]byte, BlockSize)
	if _, err := rand.Read(buf); err != nil {
		f.Close()
		os.Remove(path)
		return nil, nil, fmt.Errorf("bench: fill buffer: %w", err)
	}

	samples := make(chan Sample, 4)
	resultCh := make(chan Result, 1)

	go func() {
		defer close(samples)
		defer close(resultCh)
		defer func() {
			f.Close()
			os.Remove(path)
		}()

		start := time.Now()
		var (
			total      int64
			block      int64
			lastBlockT = start
			minSpeed   = 0.0
			maxSpeed   = 0.0
			haveMin    = false
			collected  []Sample
			cancelled  bool
		)

		emit := func(now time.Time) {
			elapsed := now.Sub(start)
			blockElapsed := now.Sub(lastBlockT).Seconds()
			if blockElapsed <= 0 {
				return
			}
			_ = f.Sync() // surface real disk speed, not page cache
			blockSpeed := float64(block) / blockElapsed
			avgSpeed := float64(total) / elapsed.Seconds()

			if blockSpeed > maxSpeed {
				maxSpeed = blockSpeed
			}
			if !haveMin || blockSpeed < minSpeed {
				minSpeed = blockSpeed
				haveMin = true
			}

			s := Sample{
				T:            now,
				Elapsed:      elapsed,
				BytesWritten: total,
				BlockBytes:   block,
				BlockSpeed:   blockSpeed,
				AvgSpeed:     avgSpeed,
			}
			collected = append(collected, s)
			select {
			case samples <- s:
			default:
				// drop live update if consumer is slow; collected[] retains it
			}
			block = 0
			lastBlockT = now
		}

	loop:
		for {
			select {
			case <-ctx.Done():
				cancelled = true
				break loop
			default:
			}

			if total >= opts.FileSize {
				break
			}

			n, werr := f.Write(buf)
			total += int64(n)
			block += int64(n)

			if werr != nil {
				if !strings.Contains(werr.Error(), "no space left on device") {
					fmt.Fprintln(os.Stderr, "\nwrite error:", werr)
				}
				break
			}

			if time.Since(lastBlockT) >= SampleRate {
				emit(time.Now())
			}
		}

		// final emit so consumers see the tail of the run
		if block > 0 && !cancelled {
			emit(time.Now())
		}

		duration := time.Since(start)
		avg := 0.0
		if duration > 0 {
			avg = float64(total) / duration.Seconds()
		}
		resultCh <- Result{
			Started:   start,
			Finished:  time.Now(),
			Dir:       opts.Dir,
			FileSize:  opts.FileSize,
			Written:   total,
			Samples:   collected,
			Min:       minSpeed,
			Max:       maxSpeed,
			Avg:       avg,
			Duration:  duration,
			Cancelled: cancelled,
		}
	}()

	return samples, resultCh, nil
}
