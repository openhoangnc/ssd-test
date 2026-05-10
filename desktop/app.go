package main

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"sync"
	"time"

	wailsrt "github.com/wailsapp/wails/v2/pkg/runtime"

	"github.com/openhoangnc/ssd-test/internal/bench"
	"github.com/openhoangnc/ssd-test/internal/clipboard"
	"github.com/openhoangnc/ssd-test/internal/format"
	"github.com/openhoangnc/ssd-test/internal/hwinfo"
	"github.com/openhoangnc/ssd-test/internal/report"
)

// version is overwritten at release time via -ldflags "-X main.version=v1.2.3".
var version = "dev"

const (
	updateRepo = "openhoangnc/ssd-test"
)

// SystemInfoJSON is the frontend-friendly representation of hwinfo.SystemInfo.
type SystemInfoJSON struct {
	OS            string `json:"os"`
	Arch          string `json:"arch"`
	CPUModel      string `json:"cpuModel"`
	CPUCores      int    `json:"cpuCores"`
	RAMBytes      int64  `json:"ramBytes"`
	RAMFormatted  string `json:"ramFormatted"`
	DiskModel     string `json:"diskModel"`
	DiskSizeBytes int64  `json:"diskSizeBytes"`
	DiskFreeBytes int64  `json:"diskFreeBytes"`
	DiskSize      string `json:"diskSize"`
	DiskFree      string `json:"diskFree"`
}

// SampleJSON is the frontend-friendly representation of bench.Sample.
type SampleJSON struct {
	Elapsed      float64 `json:"elapsed"`
	BytesWritten int64   `json:"bytesWritten"`
	BlockSpeed   float64 `json:"blockSpeed"`
	AvgSpeed     float64 `json:"avgSpeed"`
	SpeedText    string  `json:"speedText"`
	AvgText      string  `json:"avgText"`
	Progress     float64 `json:"progress"`
}

// ResultJSON is the frontend-friendly representation of bench.Result.
type ResultJSON struct {
	Written      int64   `json:"written"`
	WrittenText  string  `json:"writtenText"`
	Duration     float64 `json:"duration"`
	DurationText string  `json:"durationText"`
	Min          float64 `json:"min"`
	Max          float64 `json:"max"`
	Avg          float64 `json:"avg"`
	MinText      string  `json:"minText"`
	MaxText      string  `json:"maxText"`
	AvgText      string  `json:"avgText"`
	Cancelled    bool    `json:"cancelled"`
	CacheDetected bool   `json:"cacheDetected"`
	CacheBytes   int64   `json:"cacheBytes"`
	CacheText    string  `json:"cacheText"`
	BurstSpeed   string  `json:"burstSpeed"`
	SteadySpeed  string  `json:"steadySpeed"`
}

// UpdateInfo describes an available update.
type UpdateInfo struct {
	Available bool   `json:"available"`
	Latest    string `json:"latest"`
	Current   string `json:"current"`
	URL       string `json:"url"`
}

// App is the Wails application struct.
type App struct {
	ctx       context.Context
	cancelMu  sync.Mutex
	cancelFn  context.CancelFunc
	benchRes  *report.Result
}

// NewApp creates a new App application struct.
func NewApp() *App {
	return &App{}
}

// startup is called when the app starts.
func (a *App) startup(ctx context.Context) {
	a.ctx = ctx
}

// GetVersion returns the current app version.
func (a *App) GetVersion() string {
	return version
}

// GetSystemInfo returns system and disk info for the given path.
func (a *App) GetSystemInfo(path string) SystemInfoJSON {
	if path == "" {
		path = "."
	}
	sys := hwinfo.Collect(path)
	return SystemInfoJSON{
		OS:            sys.OS,
		Arch:          sys.Arch,
		CPUModel:      dashIfEmpty(sys.CPUModel),
		CPUCores:      sys.CPUCores,
		RAMBytes:      sys.RAMBytes,
		RAMFormatted:  format.Bytes(sys.RAMBytes),
		DiskModel:     dashIfEmpty(sys.DiskModel),
		DiskSizeBytes: sys.DiskSizeBytes,
		DiskFreeBytes: sys.DiskFreeBytes,
		DiskSize:      format.Bytes(sys.DiskSizeBytes),
		DiskFree:      format.Bytes(sys.DiskFreeBytes),
	}
}

// SelectDirectory opens a native directory picker.
func (a *App) SelectDirectory() (string, error) {
	dir, err := wailsrt.OpenDirectoryDialog(a.ctx, wailsrt.OpenDialogOptions{
		Title: "Select directory to test",
	})
	if err != nil {
		return "", err
	}
	return dir, nil
}

// ComputeTestSize returns the test file size in bytes for the given args.
func (a *App) ComputeTestSize(path, sizeArg string) (int64, error) {
	if path == "" {
		path = "."
	}
	sys := hwinfo.Collect(path)
	return computeSize(sizeArg, sys.DiskFreeBytes, sys.DiskSizeBytes)
}

// StartBenchmark runs the write benchmark and emits events to the frontend.
func (a *App) StartBenchmark(dir, sizeArg string) error {
	if dir == "" {
		dir = "."
	}
	sys := hwinfo.Collect(dir)
	fileSize, err := computeSize(sizeArg, sys.DiskFreeBytes, sys.DiskSizeBytes)
	if err != nil {
		return err
	}
	if fileSize <= 0 {
		return errors.New("not enough free space for a meaningful test")
	}

	ctx, cancel := context.WithCancel(a.ctx)
	a.cancelMu.Lock()
	a.cancelFn = cancel
	a.cancelMu.Unlock()

	samples, resultCh, err := bench.Run(ctx, bench.Options{Dir: dir, FileSize: fileSize})
	if err != nil {
		cancel()
		return err
	}

	// Emit the test size to the frontend
	wailsrt.EventsEmit(a.ctx, "bench:started", map[string]interface{}{
		"fileSize":     fileSize,
		"fileSizeText": format.Bytes(fileSize),
		"dir":          dir,
	})

	go func() {
		defer cancel()
		for s := range samples {
			wailsrt.EventsEmit(a.ctx, "bench:sample", SampleJSON{
				Elapsed:      s.Elapsed.Seconds(),
				BytesWritten: s.BytesWritten,
				BlockSpeed:   s.BlockSpeed,
				AvgSpeed:     s.AvgSpeed,
				SpeedText:    format.BytesPerSec(s.BlockSpeed),
				AvgText:      format.BytesPerSec(s.AvgSpeed),
				Progress:     float64(s.BytesWritten) / float64(fileSize) * 100,
			})
		}
		bres := <-resultCh
		cache := bench.EstimateCache(bres.Samples)
		a.benchRes = &report.Result{Sys: sys, Bench: bres}

		result := ResultJSON{
			Written:      bres.Written,
			WrittenText:  format.Bytes(bres.Written),
			Duration:     bres.Duration.Seconds(),
			DurationText: format.Duration(bres.Duration),
			Min:          bres.Min,
			Max:          bres.Max,
			Avg:          bres.Avg,
			MinText:      format.BytesPerSec(bres.Min),
			MaxText:      format.BytesPerSec(bres.Max),
			AvgText:      format.BytesPerSec(bres.Avg),
			Cancelled:    bres.Cancelled,
			CacheDetected: cache.Detected,
		}
		if cache.Detected {
			result.CacheBytes = cache.Bytes
			result.CacheText = format.Bytes(cache.Bytes)
			result.BurstSpeed = format.BytesPerSec(cache.BurstSpeed)
			result.SteadySpeed = format.BytesPerSec(cache.SteadySpeed)
		}
		wailsrt.EventsEmit(a.ctx, "bench:done", result)
	}()

	return nil
}

// CancelBenchmark cancels any running benchmark.
func (a *App) CancelBenchmark() {
	a.cancelMu.Lock()
	defer a.cancelMu.Unlock()
	if a.cancelFn != nil {
		a.cancelFn()
		a.cancelFn = nil
	}
}

// SaveHTMLReport saves the last benchmark result as an HTML file.
func (a *App) SaveHTMLReport() (string, error) {
	if a.benchRes == nil {
		return "", errors.New("no benchmark result available")
	}
	path, err := wailsrt.SaveFileDialog(a.ctx, wailsrt.SaveDialogOptions{
		Title:           "Save HTML Report",
		DefaultFilename: defaultReportPath(),
		Filters: []wailsrt.FileFilter{
			{DisplayName: "HTML files", Pattern: "*.html"},
		},
	})
	if err != nil {
		return "", err
	}
	if path == "" {
		return "", nil // user cancelled
	}
	html, err := report.HTML(*a.benchRes)
	if err != nil {
		return "", err
	}
	if err := os.WriteFile(path, []byte(html), 0o644); err != nil {
		return "", err
	}
	return path, nil
}

// CopyToClipboard copies the Markdown summary to clipboard.
func (a *App) CopyToClipboard() error {
	if a.benchRes == nil {
		return errors.New("no benchmark result available")
	}
	return clipboard.Copy(report.Markdown(*a.benchRes))
}

// CheckForUpdate checks GitHub for a newer version.
func (a *App) CheckForUpdate() UpdateInfo {
	if version == "" || version == "dev" {
		return UpdateInfo{Current: version}
	}

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()

	url := "https://api.github.com/repos/" + updateRepo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return UpdateInfo{Current: version}
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ssd-test-desktop")

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return UpdateInfo{Current: version}
	}
	defer resp.Body.Close()

	if resp.StatusCode != 200 {
		return UpdateInfo{Current: version}
	}

	var rel struct {
		TagName string `json:"tag_name"`
		HTMLURL string `json:"html_url"`
	}
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return UpdateInfo{Current: version}
	}

	newer := compareSemver(rel.TagName, version) > 0
	return UpdateInfo{
		Available: newer,
		Latest:    rel.TagName,
		Current:   version,
		URL:       rel.HTMLURL,
	}
}

// OpenURL opens a URL in the default browser.
func (a *App) OpenURL(url string) {
	wailsrt.BrowserOpenURL(a.ctx, url)
}

// ── helpers ──────────────────────────────────────────────────────────────

func defaultReportPath() string {
	ts := time.Now().Format("20060102-150405")
	return fmt.Sprintf("ssd-test-report-%s.html", ts)
}

func dashIfEmpty(s string) string {
	if s == "" {
		return "—"
	}
	return s
}

func computeSize(arg string, free, total int64) (int64, error) {
	arg = strings.TrimSpace(arg)
	if arg == "" || strings.EqualFold(arg, "auto") {
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

func compareSemver(a, b string) int {
	a = strings.TrimPrefix(a, "v")
	b = strings.TrimPrefix(b, "v")
	aMain, aPre, _ := strings.Cut(a, "-")
	bMain, bPre, _ := strings.Cut(b, "-")
	ap := strings.Split(aMain, ".")
	bp := strings.Split(bMain, ".")
	for i := 0; i < len(ap) || i < len(bp); i++ {
		var av, bv int
		if i < len(ap) {
			av, _ = strconv.Atoi(ap[i])
		}
		if i < len(bp) {
			bv, _ = strconv.Atoi(bp[i])
		}
		if av != bv {
			if av > bv {
				return 1
			}
			return -1
		}
	}
	if aPre == "" && bPre != "" {
		return 1
	}
	if aPre != "" && bPre == "" {
		return -1
	}
	if aPre < bPre {
		return -1
	}
	if aPre > bPre {
		return 1
	}
	return 0
}

// GetOSInfo returns info about the current runtime for the frontend.
func (a *App) GetOSInfo() map[string]string {
	return map[string]string{
		"os":   runtime.GOOS,
		"arch": runtime.GOARCH,
	}
}

// GetDefaultPath returns the current working directory.
func (a *App) GetDefaultPath() string {
	dir, err := os.Getwd()
	if err != nil {
		home, _ := os.UserHomeDir()
		return home
	}
	return dir
}

// FormatBytes formats a byte count for display.
func (a *App) FormatBytes(b int64) string {
	return format.Bytes(b)
}

// FormatSpeed formats a speed value for display.
func (a *App) FormatSpeed(bps float64) string {
	return format.BytesPerSec(bps)
}

// GetAbsPath returns the absolute path for a given path.
func (a *App) GetAbsPath(path string) string {
	abs, err := filepath.Abs(path)
	if err != nil {
		return path
	}
	return abs
}
