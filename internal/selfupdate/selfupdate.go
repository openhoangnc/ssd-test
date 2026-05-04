// Package selfupdate checks GitHub Releases for a newer build of ssd-test
// and replaces the running binary in place. Network failures are silent —
// the test must always run.
package selfupdate

import (
	"context"
	"crypto/sha256"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"os"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"
	"time"
)

const (
	repo            = "openhoangnc/ssd-test"
	httpTimeout     = 1500 * time.Millisecond
	downloadTimeout = 30 * time.Second
	cacheTTL        = 6 * time.Hour
	devVersion      = "dev"
)

// Result describes what happened during a check.
type Result struct {
	Checked        bool   // did we actually contact GitHub?
	Latest         string // tag of latest release, if known
	Updated        bool   // a new binary was installed
	UpdatedBinPath string // path to the new binary on disk (if updated)
	Skipped        string // reason we skipped (cached / disabled / offline / not-newer / dev)
	Err            error  // non-fatal error encountered (network, parse, etc.)
}

// Check inspects the latest release and applies an update if one is available.
// On a successful update, ApplyAndExec is invoked which does not return on
// Unix (replaces the process) and exits cleanly on Windows.
//
// version is the build's main.version (use "dev" for local builds).
// disabled (e.g. --no-update or env override) short-circuits the call.
func Check(ctx context.Context, version string, disabled bool) Result {
	if disabled {
		return Result{Skipped: "disabled by flag/env"}
	}
	if version == "" || version == devVersion {
		return Result{Skipped: "dev build"}
	}

	if cached, ok := readCache(); ok {
		// Use cached "no update" decision; if cache says there was an update
		// but it didn't get applied for some reason, fall through to recheck.
		if !cached.Newer {
			return Result{Latest: cached.LatestTag, Skipped: "cache fresh"}
		}
	}

	rel, err := fetchLatest(ctx)
	if err != nil {
		return Result{Err: err, Skipped: "fetch failed"}
	}
	writeCache(cacheEntry{
		LatestTag: rel.TagName,
		Newer:     compareSemver(rel.TagName, version) > 0,
		CheckedAt: time.Now(),
	})

	r := Result{Checked: true, Latest: rel.TagName}
	if compareSemver(rel.TagName, version) <= 0 {
		r.Skipped = "already up to date"
		return r
	}

	asset, sumsURL := pickAsset(rel)
	if asset == "" {
		r.Skipped = "no asset for this OS/arch"
		return r
	}

	dctx, cancel := context.WithTimeout(ctx, downloadTimeout)
	defer cancel()
	binPath, err := downloadAndVerify(dctx, asset, sumsURL)
	if err != nil {
		r.Err = err
		r.Skipped = "download failed"
		return r
	}
	r.Updated = true
	r.UpdatedBinPath = binPath
	return r
}

// ApplyAndExec replaces the current executable with newBin and re-execs (Unix)
// or spawns + exits (Windows). Returns only on error; otherwise the process
// has been replaced.
func ApplyAndExec(newBin string) error {
	currentExe, err := os.Executable()
	if err != nil {
		return fmt.Errorf("locate executable: %w", err)
	}
	currentExe, err = filepath.EvalSymlinks(currentExe)
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}
	return platformReplace(currentExe, newBin)
}

// ── GitHub API ───────────────────────────────────────────────────────────

type release struct {
	TagName string `json:"tag_name"`
	Assets  []struct {
		Name        string `json:"name"`
		DownloadURL string `json:"browser_download_url"`
	} `json:"assets"`
}

func fetchLatest(ctx context.Context) (*release, error) {
	url := "https://api.github.com/repos/" + repo + "/releases/latest"
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, err
	}
	req.Header.Set("Accept", "application/vnd.github+json")
	req.Header.Set("User-Agent", "ssd-test-selfupdate")

	client := &http.Client{Timeout: httpTimeout}
	resp, err := client.Do(req)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return nil, fmt.Errorf("github: status %d", resp.StatusCode)
	}
	var rel release
	if err := json.NewDecoder(resp.Body).Decode(&rel); err != nil {
		return nil, err
	}
	return &rel, nil
}

func pickAsset(rel *release) (assetURL, sumsURL string) {
	wantBin := "ssd-test-" + runtime.GOOS + "-" + runtime.GOARCH
	if runtime.GOOS == "windows" {
		wantBin += ".exe"
	}
	for _, a := range rel.Assets {
		switch a.Name {
		case wantBin:
			assetURL = a.DownloadURL
		case "SHA256SUMS":
			sumsURL = a.DownloadURL
		}
	}
	return
}

// ── download + verify ────────────────────────────────────────────────────

func downloadAndVerify(ctx context.Context, assetURL, sumsURL string) (string, error) {
	wantSum, err := fetchSum(ctx, sumsURL, assetURL)
	if err != nil {
		return "", fmt.Errorf("checksum: %w", err)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, assetURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ssd-test-selfupdate")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", fmt.Errorf("download: status %d", resp.StatusCode)
	}

	currentExe, err := os.Executable()
	if err != nil {
		return "", err
	}
	tmp, err := os.CreateTemp(filepath.Dir(currentExe), ".ssd-test.new-*")
	if err != nil {
		return "", err
	}
	defer tmp.Close()

	h := sha256.New()
	if _, err := io.Copy(io.MultiWriter(tmp, h), resp.Body); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	gotSum := hex.EncodeToString(h.Sum(nil))
	if wantSum != "" && !strings.EqualFold(gotSum, wantSum) {
		os.Remove(tmp.Name())
		return "", fmt.Errorf("sha256 mismatch: got %s want %s", gotSum, wantSum)
	}
	if err := tmp.Chmod(0o755); err != nil {
		os.Remove(tmp.Name())
		return "", err
	}
	return tmp.Name(), nil
}

func fetchSum(ctx context.Context, sumsURL, assetURL string) (string, error) {
	if sumsURL == "" {
		return "", nil // checksum file is optional; we'll skip verification
	}
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, sumsURL, nil)
	if err != nil {
		return "", err
	}
	req.Header.Set("User-Agent", "ssd-test-selfupdate")
	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return "", err
	}
	defer resp.Body.Close()
	if resp.StatusCode != 200 {
		return "", nil
	}
	body, err := io.ReadAll(io.LimitReader(resp.Body, 64*1024))
	if err != nil {
		return "", err
	}
	asset := assetURL[strings.LastIndex(assetURL, "/")+1:]
	for line := range strings.Lines(string(body)) {
		fields := strings.Fields(strings.TrimRight(line, "\n"))
		if len(fields) < 2 {
			continue
		}
		name := strings.TrimPrefix(fields[1], "*")
		if name == asset {
			return fields[0], nil
		}
	}
	return "", errors.New("sum not listed for asset")
}

// ── semver compare ───────────────────────────────────────────────────────

// compareSemver returns -1/0/+1 by comparing dotted integer fields. Strips
// leading "v". Pre-release suffixes after "-" are compared lexically.
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
	// no pre-release > pre-release
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

// ── cache ─────────────────────────────────────────────────────────────────

type cacheEntry struct {
	LatestTag string    `json:"latest_tag"`
	Newer     bool      `json:"newer"`
	CheckedAt time.Time `json:"checked_at"`
}

func cacheFile() string {
	dir := os.Getenv("XDG_CACHE_HOME")
	if dir == "" {
		home, err := os.UserHomeDir()
		if err != nil {
			return ""
		}
		dir = filepath.Join(home, ".cache")
	}
	return filepath.Join(dir, "ssd-test", "update-check.json")
}

func readCache() (cacheEntry, bool) {
	path := cacheFile()
	if path == "" {
		return cacheEntry{}, false
	}
	data, err := os.ReadFile(path)
	if err != nil {
		return cacheEntry{}, false
	}
	var e cacheEntry
	if err := json.Unmarshal(data, &e); err != nil {
		return cacheEntry{}, false
	}
	if time.Since(e.CheckedAt) > cacheTTL {
		return cacheEntry{}, false
	}
	return e, true
}

func writeCache(e cacheEntry) {
	path := cacheFile()
	if path == "" {
		return
	}
	_ = os.MkdirAll(filepath.Dir(path), 0o755)
	data, err := json.Marshal(e)
	if err != nil {
		return
	}
	_ = os.WriteFile(path, data, 0o644)
}
