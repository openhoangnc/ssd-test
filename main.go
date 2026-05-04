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
		output   = flag.String("output", "", "write report to this file (.html, .svg, or .md)")
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
	useTUI := !simple && tui.IsTTY() && stdinIsTTY()

	sys := hwinfo.Collect(dir)
	fileSize, err := computeSize(sizeArg, sys.DiskFreeBytes, sys.DiskSizeBytes)
	if err != nil {
		return err
	}
	if fileSize <= 0 {
		return errors.New("not enough free space for a meaningful test")
	}

	if useTUI {
		return runTUI(ctx, dir, fileSize, sys, output, copyOut)
	}
	return runInline(ctx, dir, fileSize, sys, output, copyOut, jsonOut)
}

// runTUI is the interactive flow: alt-screen + raw input, with a confirmation
// step before the test starts and an action menu after it completes.
func runTUI(ctx context.Context, dir string, fileSize int64, sys hwinfo.SystemInfo, output string, copyOut bool) error {
	raw, err := tui.EnterRaw()
	if err != nil {
		return fmt.Errorf("raw mode: %w", err)
	}
	defer raw.Restore()

	screen := tui.Enter(os.Stdout)
	defer screen.Restore()

	// 1. Confirm screen.
	confirmFrame := tui.Frame{
		Phase:    tui.PhaseConfirm,
		Sys:      sys,
		Dir:      dir,
		FileSize: fileSize,
	}
	screen.Render(confirmFrame)
	if !waitForKey(ctx, "\r\nqQ") {
		return nil
	}
	if lastKey == 'q' || lastKey == 'Q' {
		return nil
	}

	// 2. Run the bench loop, redrawing on each sample.
	samples, resultCh, err := bench.Run(ctx, bench.Options{Dir: dir, FileSize: fileSize})
	if err != nil {
		return err
	}
	history := make([]float64, 0, 600)
	var maxSpeed, minSpeed float64
	haveMin := false
	var lastSample bench.Sample

	for s := range samples {
		history = append(history, s.BlockSpeed)
		if s.BlockSpeed > maxSpeed {
			maxSpeed = s.BlockSpeed
		}
		if !haveMin || s.BlockSpeed < minSpeed {
			minSpeed = s.BlockSpeed
			haveMin = true
		}
		lastSample = s
		screen.Render(tui.Frame{
			Phase:    tui.PhaseRunning,
			Sys:      sys,
			Dir:      dir,
			FileSize: fileSize,
			Sample:   s,
			History:  history,
			Min:      minSpeed,
			Max:      maxSpeed,
		})
	}
	bres := <-resultCh

	r := report.Result{Sys: sys, Bench: bres}

	// Apply any non-interactive flags before the menu so they always run.
	var status string
	if output != "" {
		if werr := writeReport(r, output); werr != nil {
			status = "save failed: " + werr.Error()
		} else {
			status = "Saved " + output
		}
	}
	if copyOut {
		if cerr := clipboard.Copy(report.Markdown(r)); cerr != nil {
			status = "clipboard: " + cerr.Error()
		} else {
			status = "Copied summary to clipboard."
		}
	}

	// 3. Action loop.
	for {
		screen.Render(tui.Frame{
			Phase:    tui.PhaseDone,
			Sys:      sys,
			Dir:      dir,
			FileSize: fileSize,
			Sample:   lastSample,
			History:  history,
			Min:      minSpeed,
			Max:      maxSpeed,
			Result:   bres,
			Status:   status,
		})
		if !waitForKey(ctx, "cChHqQ\x03") {
			return nil
		}
		switch lastKey {
		case 'c', 'C':
			if err := clipboard.Copy(report.Markdown(r)); err != nil {
				status = "clipboard: " + err.Error()
			} else {
				status = "Copied summary to clipboard."
			}
		case 'h', 'H':
			path := defaultReportPath()
			if err := writeReport(r, path); err != nil {
				status = "save failed: " + err.Error()
			} else {
				status = "Saved " + path
			}
		case 'q', 'Q', '\x03':
			return nil
		}
	}
}

// runInline is the non-TTY / --simple / --json path: same as before,
// with no confirmation prompt and no menu.
func runInline(ctx context.Context, dir string, fileSize int64, sys hwinfo.SystemInfo, output string, copyOut, jsonOut bool) error {
	if !jsonOut {
		fmt.Printf("Device:   %s\n", dashIfEmpty(sys.DiskModel))
		fmt.Printf("Disk:     %s total, %s free\n",
			format.Bytes(sys.DiskSizeBytes), format.Bytes(sys.DiskFreeBytes))
		fmt.Printf("Test:     writing %s to %s\n", format.Bytes(fileSize), dir)
		fmt.Println()
	}

	samples, resultCh, err := bench.Run(ctx, bench.Options{Dir: dir, FileSize: fileSize})
	if err != nil {
		return err
	}
	for s := range samples {
		if jsonOut {
			continue
		}
		pct := float64(s.BytesWritten) / float64(fileSize) * 100
		fmt.Printf("\r\x1b[K%s/%s (%.1f%%)  current %s  avg %s",
			format.Bytes(s.BytesWritten), format.Bytes(fileSize), pct,
			format.BytesPerSec(s.BlockSpeed),
			format.BytesPerSec(s.AvgSpeed))
	}
	bres := <-resultCh
	if !jsonOut {
		fmt.Println()
	}
	r := report.Result{Sys: sys, Bench: bres}

	if jsonOut {
		return emitJSON(r)
	}
	printSummary(r)

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

// lastKey holds the most recent key returned by waitForKey, so callers can
// switch on it without juggling extra return values.
var lastKey byte

// waitForKey blocks until the user presses one of the bytes in `accept` or
// the context is cancelled. Returns true if a key was read, false if context
// cancelled. Result is in lastKey.
func waitForKey(ctx context.Context, accept string) bool {
	keyCh := make(chan byte, 1)
	errCh := make(chan error, 1)
	go func() {
		for {
			k, err := tui.ReadKey()
			if err != nil {
				errCh <- err
				return
			}
			if strings.IndexByte(accept, k) >= 0 {
				keyCh <- k
				return
			}
			// Ignore keys not in the accept set (lets the user mash random
			// keys without exiting menus).
		}
	}()
	select {
	case k := <-keyCh:
		lastKey = k
		return true
	case <-errCh:
		return false
	case <-ctx.Done():
		return false
	}
}

func defaultReportPath() string {
	ts := time.Now().Format("20060102-150405")
	return fmt.Sprintf("ssd-test-report-%s.html", ts)
}

func stdinIsTTY() bool {
	stat, err := os.Stdin.Stat()
	if err != nil {
		return false
	}
	return (stat.Mode() & os.ModeCharDevice) != 0
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
	case ".svg":
		return os.WriteFile(path, []byte(report.SVG(r)), 0o644)
	case ".md":
		return os.WriteFile(path, []byte(report.Markdown(r)), 0o644)
	default:
		return fmt.Errorf("unsupported output extension: %s (use .html, .svg, or .md)",
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
