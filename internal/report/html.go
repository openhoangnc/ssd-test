package report

import (
	"bytes"
	"fmt"
	"html/template"
	"time"

	"github.com/openhoangnc/ssd-test/internal/bench"
	"github.com/openhoangnc/ssd-test/internal/format"
)

// HTML renders a self-contained HTML report (inline CSS, inline SVG). The
// caller can write the result directly to a .html file.
func HTML(r Result) (string, error) {
	cache := bench.EstimateCache(r.Bench.Samples)
	tpl, err := template.New("report").Funcs(template.FuncMap{
		"bytes":    func(b int64) string { return format.Bytes(b) },
		"bytesps":  func(f float64) string { return format.BytesPerSec(f) },
		"duration": func(d time.Duration) string { return format.Duration(d) },
		"dash":     dashIfEmpty,
		"chart":    func() template.HTML { return template.HTML(SVG(r)) },
		"time": func(t time.Time) string {
			return t.Format("2006-01-02 15:04:05 MST")
		},
	}).Parse(htmlTemplate)
	if err != nil {
		return "", fmt.Errorf("parse template: %w", err)
	}
	var buf bytes.Buffer
	data := struct {
		Result
		Cache bench.CacheEstimate
	}{r, cache}
	if err := tpl.Execute(&buf, data); err != nil {
		return "", fmt.Errorf("execute template: %w", err)
	}
	return buf.String(), nil
}

const htmlTemplate = `<!doctype html>
<html lang="en">
<head>
<meta charset="utf-8">
<meta name="viewport" content="width=device-width, initial-scale=1">
<title>SSD Test report — {{dash .Sys.DiskModel}}</title>
<style>
:root{
  --bg:#0d1117;--card:#161b22;--fg:#c9d1d9;--muted:#8b949e;--accent:#58a6ff;
  --good:#3fb950;--warn:#d29922;--bad:#f85149;--border:#30363d;
}
@media (prefers-color-scheme: light){
  :root{--bg:#ffffff;--card:#f6f8fa;--fg:#1f2328;--muted:#57606a;--accent:#0969da;
  --good:#1a7f37;--warn:#9a6700;--bad:#cf222e;--border:#d0d7de;}
}
*{box-sizing:border-box}
body{margin:0;padding:32px;font:14px/1.5 ui-sans-serif,system-ui,-apple-system,Segoe UI,Roboto,sans-serif;background:var(--bg);color:var(--fg)}
.wrap{max-width:960px;margin:0 auto}
h1{font-size:22px;margin:0 0 4px}
.meta{color:var(--muted);font-size:13px;margin-bottom:24px}
.grid{display:grid;grid-template-columns:repeat(auto-fit,minmax(200px,1fr));gap:12px;margin-bottom:24px}
.stat{background:var(--card);border:1px solid var(--border);border-radius:10px;padding:14px}
.stat .k{color:var(--muted);font-size:12px;text-transform:uppercase;letter-spacing:.04em}
.stat .v{font-size:20px;font-weight:600;margin-top:4px}
.stat.good .v{color:var(--good)}
.stat.bad .v{color:var(--bad)}
.stat.warn .v{color:var(--warn)}
.card{background:var(--card);border:1px solid var(--border);border-radius:12px;padding:18px;margin-bottom:18px}
.card h2{font-size:14px;text-transform:uppercase;letter-spacing:.06em;color:var(--muted);margin:0 0 12px}
table{width:100%;border-collapse:collapse;font-size:13px}
th,td{text-align:left;padding:6px 8px;border-bottom:1px solid var(--border)}
th{color:var(--muted);font-weight:500;font-size:12px;text-transform:uppercase;letter-spacing:.04em}
.chart-card svg{width:100%;height:auto;display:block}
footer{color:var(--muted);font-size:12px;text-align:center;margin-top:24px}
a{color:var(--accent)}
</style>
</head>
<body>
<div class="wrap">
  <h1>SSD Test — {{dash .Sys.DiskModel}}</h1>
  <div class="meta">{{time .Sys.Timestamp}} · {{.Sys.OS}}/{{.Sys.Arch}}</div>

  <div class="grid">
    <div class="stat good"><div class="k">Average</div><div class="v">{{bytesps .Bench.Avg}}</div></div>
    <div class="stat good"><div class="k">Max</div><div class="v">{{bytesps .Bench.Max}}</div></div>
    <div class="stat bad"><div class="k">Min</div><div class="v">{{bytesps .Bench.Min}}</div></div>
    <div class="stat"><div class="k">Written</div><div class="v">{{bytes .Bench.Written}}</div></div>
    <div class="stat"><div class="k">Duration</div><div class="v">{{duration .Bench.Duration}}</div></div>
    {{if .Cache.Detected}}<div class="stat warn" title="Approximate fast-cache (SLC/DRAM) capacity, inferred from the speed cliff."><div class="k">Cache estimate</div><div class="v">~{{bytes .Cache.Bytes}}</div></div>{{end}}
  </div>

  <div class="card chart-card">
    <h2>Speed over time</h2>
    {{chart}}
  </div>

  <div class="card">
    <h2>System</h2>
    <table>
      <tr><th>Device</th><td>{{dash .Sys.DiskModel}}</td></tr>
      <tr><th>Disk size</th><td>{{bytes .Sys.DiskSizeBytes}}</td></tr>
      <tr><th>Free space</th><td>{{bytes .Sys.DiskFreeBytes}}</td></tr>
      <tr><th>CPU</th><td>{{dash .Sys.CPUModel}} · {{.Sys.CPUCores}} cores</td></tr>
      <tr><th>RAM</th><td>{{bytes .Sys.RAMBytes}}</td></tr>
      <tr><th>OS</th><td>{{.Sys.OS}}/{{.Sys.Arch}}</td></tr>
    </table>
  </div>

  <div class="card">
    <h2>Test parameters</h2>
    <table>
      <tr><th>Target directory</th><td>{{.Bench.Dir}}</td></tr>
      <tr><th>Requested size</th><td>{{bytes .Bench.FileSize}}</td></tr>
      <tr><th>Bytes written</th><td>{{bytes .Bench.Written}}</td></tr>
      <tr><th>Started</th><td>{{time .Bench.Started}}</td></tr>
      <tr><th>Finished</th><td>{{time .Bench.Finished}}</td></tr>
      {{if .Bench.Cancelled}}<tr><th>Status</th><td>Cancelled before completion</td></tr>{{end}}
    </table>
  </div>

  <footer>
    Generated by <a href="https://github.com/openhoangnc/ssd-test">ssd-test</a> · self-contained, no external assets
  </footer>
</div>
</body>
</html>
`
