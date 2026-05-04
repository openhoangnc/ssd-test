package main

import (
	"context"
	"encoding/json"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"strconv"
	"strings"
	"syscall"
	"time"

	"github.com/openhoangnc/ssd-test/internal/bench"
	"github.com/openhoangnc/ssd-test/internal/clipboard"
	"github.com/openhoangnc/ssd-test/internal/format"
	"github.com/openhoangnc/ssd-test/internal/hwinfo"
	"github.com/openhoangnc/ssd-test/internal/report"
	"github.com/openhoangnc/ssd-test/internal/selfupdate"
	"github.com/openhoangnc/ssd-test/internal/tui"
)

// version is overwritten at release time via -ldflags "-X main.version=v1.2.3".
var version = "dev"

func main() {
	selfupdate.CleanupStaleOld()

	var (
		path     = flag.String("path", ".", "directory to test on")
		sizeFlag = flag.String("size", "auto", "test size (auto, 200M, 2G, 50%)")
		output   = flag.String("output", "", "write report to this file (.html or .png)")
		copyOut  = flag.Bool("copy", false, "copy a Markdown summary to the clipboard")
		simple   = flag.Bool("simple", false, "use inline output instead of full-screen TUI")
		jsonOut  = flag.Bool("json", false, "emit JSON result to stdout (implies --simple)")
		noUpdate = flag.Bool("no-update", false, "skip the self-update check")
		showVer  = flag.Bool("version", false, "print version and exit")
	)
	flag.Usage = func() {
		fmt.Fprintf(os.Stderr, "ssd-test %s — measure sustained SSD write speed\n\n", version)
		fmt.Fprintf(os.Stderr, "Usage: ssd-test [flags]\n\nFlags:\n")
		flag.PrintDefaults()
	}
	flag.Parse()

	if *showVer {
		fmt.Println("ssd-test", version)
		return
	}

	// Self-update only fires on a bare invocation. Any flag → skip.
	bareInvocation := flag.NFlag() == 0
	updateDisabled := *noUpdate || os.Getenv("SSD_TEST_NO_UPDATE") == "1" || !bareInvocation

	if bareInvocation && !updateDisabled && tui.IsTTY() {
		ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
		res := selfupdate.Check(ctx, version, false)
		cancel()
		if res.Updated {
			fmt.Fprintf(os.Stderr, "Updated to %s, restarting...\n", res.Latest)
			if err := selfupdate.ApplyAndExec(res.UpdatedBinPath); err != nil {
				fmt.Fprintln(os.Stderr, "self-update failed:", err)
				// fall through and run the test with the current binary
			}
		}
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	if err := run(ctx, *path, *sizeFlag, *output, *copyOut, *simple, *jsonOut); err != nil {
		fmt.Fprintln(os.Stderr, "error:", err)
		os.Exit(1)
	}
}

func run(ctx context.Context, dir, sizeArg, output string, copyOut, simple, jsonOut bool) error {
	if jsonOut {
		simple = true
	}
	useTUI := !simple && tui.IsTTY()

	sys := hwinfo.Collect(dir)
	fileSize, err := computeSize(sizeArg, sys.DiskFreeBytes, sys.DiskSizeBytes)
	if err != nil {
		return err
	}
	if fileSize <= 0 {
		return errors.New("not enough free space for a meaningful test")
	}

	if !useTUI && !jsonOut {
		fmt.Printf("Device:   %s\n", dashIfEmpty(sys.DiskModel))
		fmt.Printf("Disk:     %s total, %s free\n",
			format.Bytes(sys.DiskSizeBytes), format.Bytes(sys.DiskFreeBytes))
		fmt.Printf("Test:     writing %s to %s\n",
			format.Bytes(fileSize), dir)
		fmt.Println()
	}

	samples, resultCh, err := bench.Run(ctx, bench.Options{Dir: dir, FileSize: fileSize})
	if err != nil {
		return err
	}

	var screen *tui.Screen
	if useTUI {
		screen = tui.Enter(os.Stdout)
		defer screen.Restore()
	}

	history := make([]float64, 0, 600)
	var last bench.Sample
	var maxSpeed, minSpeed float64
	haveMin := false

	render := func(s bench.Sample) {
		history = append(history, s.BlockSpeed)
		if s.BlockSpeed > maxSpeed {
			maxSpeed = s.BlockSpeed
		}
		if !haveMin || s.BlockSpeed < minSpeed {
			minSpeed = s.BlockSpeed
			haveMin = true
		}
		if useTUI {
			screen.Render(tui.Frame{
				Sys:      sys,
				FileSize: fileSize,
				Sample:   s,
				History:  history,
				Min:      minSpeed,
				Max:      maxSpeed,
			})
		} else if !jsonOut {
			pct := float64(s.BytesWritten) / float64(fileSize) * 100
			fmt.Printf("\r\x1b[K%s/%s (%.1f%%)  current %s  avg %s",
				format.Bytes(s.BytesWritten), format.Bytes(fileSize), pct,
				format.BytesPerSec(s.BlockSpeed),
				format.BytesPerSec(s.AvgSpeed))
		}
		last = s
	}

	for s := range samples {
		render(s)
	}
	_ = last

	bres := <-resultCh

	if screen != nil {
		screen.Restore()
	}
	if !useTUI && !jsonOut {
		fmt.Println()
	}

	r := report.Result{Sys: sys, Bench: bres}

	if jsonOut {
		return emitJSON(r)
	}

	if !jsonOut {
		printSummary(r)
	}

	if output != "" {
		if err := writeReport(r, output); err != nil {
			return fmt.Errorf("write report: %w", err)
		}
		fmt.Printf("Report written to %s\n", output)
	}
	if copyOut {
		if err := clipboard.Copy(report.Markdown(r)); err != nil {
			fmt.Fprintln(os.Stderr, "clipboard:", err)
		} else {
			fmt.Println("Summary copied to clipboard.")
		}
	}
	return nil
}

func computeSize(arg string, free, total int64) (int64, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" || strings.EqualFold(arg, "auto") {
		// leave 1% of disk size, capped at 1 GiB
		leave := total / 100
		if leave > 1<<30 {
			leave = 1 << 30
		}
		return free - leave, nil
	}
	if strings.HasSuffix(arg, "%") {
		v, err := strconv.ParseFloat(strings.TrimSuffix(arg, "%"), 64)
		if err != nil {
			return 0, fmt.Errorf("invalid size %q: %w", arg, err)
		}
		return int64(float64(free) * v / 100), nil
	}
	return parseBytes(arg)
}

func parseBytes(s string) (int64, error) {
	s = strings.TrimSpace(s)
	mult := int64(1)
	switch {
	case strings.HasSuffix(s, "T"), strings.HasSuffix(s, "TiB"), strings.HasSuffix(s, "TB"):
		mult = 1 << 40
	case strings.HasSuffix(s, "G"), strings.HasSuffix(s, "GiB"), strings.HasSuffix(s, "GB"):
		mult = 1 << 30
	case strings.HasSuffix(s, "M"), strings.HasSuffix(s, "MiB"), strings.HasSuffix(s, "MB"):
		mult = 1 << 20
	case strings.HasSuffix(s, "K"), strings.HasSuffix(s, "KiB"), strings.HasSuffix(s, "KB"):
		mult = 1 << 10
	}
	num := strings.TrimRightFunc(s, func(r rune) bool {
		return r < '0' || (r > '9' && r != '.')
	})
	v, err := strconv.ParseFloat(num, 64)
	if err != nil {
		return 0, fmt.Errorf("invalid size %q: %w", s, err)
	}
	return int64(v * float64(mult)), nil
}

func writeReport(r report.Result, path string) error {
	switch strings.ToLower(filepath.Ext(path)) {
	case ".html", ".htm":
		html, err := report.HTML(r)
		if err != nil {
			return err
		}
		return os.WriteFile(path, []byte(html), 0o644)
	case ".png":
		data, err := report.PNG(r)
		if err != nil {
			return err
		}
		return os.WriteFile(path, data, 0o644)
	case ".svg":
		return os.WriteFile(path, []byte(report.SVG(r)), 0o644)
	case ".md":
		return os.WriteFile(path, []byte(report.Markdown(r)), 0o644)
	default:
		return fmt.Errorf("unsupported output extension: %s (use .html, .png, .svg, or .md)",
			filepath.Ext(path))
	}
}

func emitJSON(r report.Result) error {
	enc := json.NewEncoder(os.Stdout)
	enc.SetIndent("", "  ")
	return enc.Encode(struct {
		System hwinfo.SystemInfo `json:"system"`
		Bench  bench.Result      `json:"bench"`
	}{r.Sys, r.Bench})
}

func printSummary(r report.Result) {
	fmt.Println()
	fmt.Printf("Device:   %s\n", dashIfEmpty(r.Sys.DiskModel))
	fmt.Printf("Written:  %s in %s\n",
		format.Bytes(r.Bench.Written), format.Duration(r.Bench.Duration))
	fmt.Printf("Speed:    min %s · avg %s · max %s\n",
		format.BytesPerSec(r.Bench.Min),
		format.BytesPerSec(r.Bench.Avg),
		format.BytesPerSec(r.Bench.Max))
	if r.Bench.Cancelled {
		fmt.Println("Status:   cancelled before completion")
	}
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}
