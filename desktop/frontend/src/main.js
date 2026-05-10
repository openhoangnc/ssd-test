import './style.css';

// ── Wails bindings ──────────────────────────────────────────────────────
const go = window.go?.main?.App;
const runtime = window.runtime;

// ── State ───────────────────────────────────────────────────────────────
let state = {
  phase: 'setup', // setup | running | done
  sysInfo: null,
  path: '',
  sizeArg: 'auto',
  samples: [],
  currentSpeed: 0,
  avgSpeed: 0,
  minSpeed: Infinity,
  maxSpeed: 0,
  progress: 0,
  result: null,
  fileSize: 0,
  fileSizeText: '',
};

let chart = null;

// ── Render ──────────────────────────────────────────────────────────────

function render() {
  const app = document.getElementById('app');
  app.innerHTML = `
    <div class="drag-bar"></div>
    <div class="header">
      <div class="header-left">
        <div class="header-icon">⚡</div>
        <h1>SSD Test</h1>
      </div>
      <span class="header-version" id="versionBadge">—</span>
    </div>
    <div class="update-banner" id="updateBanner">
      <span class="update-dot"></span>
      <span>A newer version is available:</span>
      <a id="updateLink" href="#">—</a>
    </div>

    <!-- Setup Phase -->
    <div class="phase ${state.phase === 'setup' ? 'active' : ''}" id="phaseSetup">
      <div class="card">
        <div class="card-title">System Information</div>
        <div class="sys-grid" id="sysGrid">
          <div class="sys-item"><span class="sys-label">Loading...</span><span class="sys-value">—</span></div>
        </div>
      </div>

      <div class="card config-section">
        <div class="card-title">Test Configuration</div>
        <div style="display:flex;flex-direction:column;gap:12px">
          <div class="config-group">
            <label>Target Directory</label>
            <div class="config-input-row">
              <input type="text" id="pathInput" placeholder="Current directory" value="${escHtml(state.path)}">
              <button class="btn-browse" id="btnBrowse">Browse</button>
            </div>
          </div>
          <div class="config-group">
            <label>Test Size</label>
            <select id="sizeSelect">
              <option value="auto" ${state.sizeArg === 'auto' ? 'selected' : ''}>Auto (fill free space)</option>
              <option value="50%" ${state.sizeArg === '50%' ? 'selected' : ''}>50% of free space</option>
              <option value="25%" ${state.sizeArg === '25%' ? 'selected' : ''}>25% of free space</option>
              <option value="10G" ${state.sizeArg === '10G' ? 'selected' : ''}>10 GiB</option>
              <option value="5G" ${state.sizeArg === '5G' ? 'selected' : ''}>5 GiB</option>
              <option value="2G" ${state.sizeArg === '2G' ? 'selected' : ''}>2 GiB</option>
              <option value="1G" ${state.sizeArg === '1G' ? 'selected' : ''}>1 GiB</option>
              <option value="500M" ${state.sizeArg === '500M' ? 'selected' : ''}>500 MiB</option>
              <option value="200M" ${state.sizeArg === '200M' ? 'selected' : ''}>200 MiB</option>
            </select>
          </div>
        </div>
      </div>

      <div class="warning-banner">
        <svg width="16" height="16" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2">
          <path d="M10.29 3.86L1.82 18a2 2 0 0 0 1.71 3h16.94a2 2 0 0 0 1.71-3L13.71 3.86a2 2 0 0 0-3.42 0z"/>
          <line x1="12" y1="9" x2="12" y2="13"/><line x1="12" y1="17" x2="12.01" y2="17"/>
        </svg>
        <span>This test writes a large amount of data and consumes SSD endurance (TBW). A single run is fine on a healthy drive.</span>
      </div>

      <div class="actions">
        <button class="btn-primary" id="btnStart">
          ⚡ Start Benchmark
        </button>
      </div>
    </div>

    <!-- Running Phase -->
    <div class="phase ${state.phase === 'running' ? 'active' : ''}" id="phaseRunning">
      <div class="card speed-dashboard">
        <div class="speed-hero">
          <div class="speed-value">
            <div class="speed-number" id="currentSpeed">0</div>
            <div class="speed-unit" id="currentSpeedUnit">MiB/s</div>
            <div class="speed-label">Current Speed</div>
          </div>
          <div class="speed-stats">
            <div class="stat-item">
              <div class="stat-val green" id="statAvg">—</div>
              <div class="stat-label">Average</div>
            </div>
            <div class="stat-item">
              <div class="stat-val cyan" id="statMax">—</div>
              <div class="stat-label">Max</div>
            </div>
            <div class="stat-item">
              <div class="stat-val red" id="statMin">—</div>
              <div class="stat-label">Min</div>
            </div>
          </div>
        </div>
        <div class="chart-container">
          <canvas id="speedChart"></canvas>
        </div>
      </div>

      <div class="progress-section">
        <div class="progress-info">
          <span class="progress-text" id="progressText">0 B / 0 B</span>
          <span class="progress-pct" id="progressPct">0%</span>
        </div>
        <div class="progress-bar">
          <div class="progress-fill" id="progressFill" style="width: 0%"></div>
        </div>
      </div>

      <div class="actions">
        <button class="btn-danger" id="btnCancel">Cancel</button>
      </div>
    </div>

    <!-- Done Phase -->
    <div class="phase ${state.phase === 'done' ? 'active' : ''}" id="phaseDone">
      <div class="card">
        <div class="card-title">Benchmark Results</div>
        ${state.result?.cancelled ? '<div class="cancelled-badge">⚠ Cancelled before completion</div>' : ''}
        <div class="results-grid" style="margin-top: ${state.result?.cancelled ? '12px' : '0'}">
          <div class="result-card green">
            <div class="rc-value" id="resAvg">${state.result?.avgText || '—'}</div>
            <div class="rc-label">Average</div>
          </div>
          <div class="result-card cyan">
            <div class="rc-value" id="resMax">${state.result?.maxText || '—'}</div>
            <div class="rc-label">Max</div>
          </div>
          <div class="result-card red">
            <div class="rc-value" id="resMin">${state.result?.minText || '—'}</div>
            <div class="rc-label">Min</div>
          </div>
          <div class="result-card accent">
            <div class="rc-value">${state.result?.writtenText || '—'}</div>
            <div class="rc-label">Written</div>
          </div>
          <div class="result-card">
            <div class="rc-value">${state.result?.durationText || '—'}</div>
            <div class="rc-label">Duration</div>
          </div>
          <div class="result-card amber">
            <div class="rc-value">${state.result?.cacheDetected ? '~' + state.result.cacheText : 'N/A'}</div>
            <div class="rc-label">SLC Cache</div>
          </div>
        </div>
        ${state.result?.cacheDetected ? `
          <div class="cache-info">
            ℹ Cache detected: burst ${state.result.burstSpeed} → steady ${state.result.steadySpeed}
          </div>
        ` : ''}
      </div>

      <div class="card">
        <div class="card-title">Speed Over Time</div>
        <div class="chart-container" style="height: 160px;">
          <canvas id="resultChart"></canvas>
        </div>
      </div>

      <div class="result-actions">
        <button class="btn-secondary" id="btnSaveHTML">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M21 15v4a2 2 0 01-2 2H5a2 2 0 01-2-2v-4"/><polyline points="7 10 12 15 17 10"/><line x1="12" y1="15" x2="12" y2="3"/></svg>
          Save HTML Report
        </button>
        <button class="btn-secondary" id="btnCopy">
          <svg width="14" height="14" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><rect x="9" y="9" width="13" height="13" rx="2" ry="2"/><path d="M5 15H4a2 2 0 01-2-2V4a2 2 0 012-2h9a2 2 0 012 2v1"/></svg>
          Copy Summary
        </button>
        <button class="btn-primary" id="btnNewTest" style="flex: none; padding: 10px 20px; font-size: 13px;">
          New Test
        </button>
      </div>
    </div>

    <div class="toast" id="toast"></div>
  `;

  bindEvents();
  loadSystemInfo();

  if (state.phase === 'running') {
    initChart('speedChart');
    redrawChart();
  }
  if (state.phase === 'done') {
    requestAnimationFrame(() => {
      initChart('resultChart');
      redrawChart();
    });
  }
}

// ── Events ──────────────────────────────────────────────────────────────

function bindEvents() {
  const btnBrowse = document.getElementById('btnBrowse');
  const btnStart = document.getElementById('btnStart');
  const btnCancel = document.getElementById('btnCancel');
  const btnSaveHTML = document.getElementById('btnSaveHTML');
  const btnCopy = document.getElementById('btnCopy');
  const btnNewTest = document.getElementById('btnNewTest');
  const pathInput = document.getElementById('pathInput');
  const sizeSelect = document.getElementById('sizeSelect');

  if (btnBrowse) btnBrowse.onclick = async () => {
    try {
      const dir = await go.SelectDirectory();
      if (dir) {
        state.path = dir;
        pathInput.value = dir;
        loadSystemInfo();
      }
    } catch (e) { console.error(e); }
  };

  if (pathInput) pathInput.onchange = () => {
    state.path = pathInput.value;
    loadSystemInfo();
  };

  if (sizeSelect) sizeSelect.onchange = () => {
    state.sizeArg = sizeSelect.value;
  };

  if (btnStart) btnStart.onclick = startBenchmark;
  if (btnCancel) btnCancel.onclick = cancelBenchmark;
  if (btnSaveHTML) btnSaveHTML.onclick = saveReport;
  if (btnCopy) btnCopy.onclick = copyToClipboard;
  if (btnNewTest) btnNewTest.onclick = resetToSetup;
}

// ── System Info ─────────────────────────────────────────────────────────

async function loadSystemInfo() {
  if (!go) return;
  try {
    const info = await go.GetSystemInfo(state.path || '.');
    state.sysInfo = info;
    renderSysGrid(info);
  } catch (e) {
    console.error('Failed to load system info:', e);
  }
}

function renderSysGrid(info) {
  const grid = document.getElementById('sysGrid');
  if (!grid) return;
  grid.innerHTML = `
    <div class="sys-item"><span class="sys-label">Disk</span><span class="sys-value">${escHtml(info.diskModel)}</span></div>
    <div class="sys-item"><span class="sys-label">Capacity</span><span class="sys-value">${info.diskSize}</span></div>
    <div class="sys-item"><span class="sys-label">Free</span><span class="sys-value">${info.diskFree}</span></div>
    <div class="sys-item"><span class="sys-label">CPU</span><span class="sys-value">${escHtml(info.cpuModel)}</span></div>
    <div class="sys-item"><span class="sys-label">Cores</span><span class="sys-value">${info.cpuCores}</span></div>
    <div class="sys-item"><span class="sys-label">RAM</span><span class="sys-value">${info.ramFormatted}</span></div>
    <div class="sys-item"><span class="sys-label">OS</span><span class="sys-value">${info.os}/${info.arch}</span></div>
  `;
}

// ── Benchmark Control ───────────────────────────────────────────────────

async function startBenchmark() {
  if (!go) return;
  state.samples = [];
  state.currentSpeed = 0;
  state.avgSpeed = 0;
  state.minSpeed = Infinity;
  state.maxSpeed = 0;
  state.progress = 0;
  state.result = null;

  const btn = document.getElementById('btnStart');
  if (btn) {
    btn.disabled = true;
    btn.innerHTML = '<span class="spinner"></span> Starting...';
  }

  try {
    await go.StartBenchmark(state.path || '.', state.sizeArg);
    state.phase = 'running';
    render();
  } catch (e) {
    showToast('Error: ' + e);
    if (btn) {
      btn.disabled = false;
      btn.innerHTML = '⚡ Start Benchmark';
    }
  }
}

async function cancelBenchmark() {
  if (!go) return;
  try {
    await go.CancelBenchmark();
  } catch (e) {
    console.error(e);
  }
}

async function saveReport() {
  if (!go) return;
  try {
    const path = await go.SaveHTMLReport();
    if (path) showToast('Saved: ' + path);
  } catch (e) {
    showToast('Error: ' + e);
  }
}

async function copyToClipboard() {
  if (!go) return;
  try {
    await go.CopyToClipboard();
    showToast('Summary copied to clipboard');
  } catch (e) {
    showToast('Error: ' + e);
  }
}

function resetToSetup() {
  state.phase = 'setup';
  state.samples = [];
  state.result = null;
  render();
}

// ── Chart ───────────────────────────────────────────────────────────────

function initChart(canvasId) {
  const canvas = document.getElementById(canvasId);
  if (!canvas) return;
  chart = {
    canvas,
    ctx: canvas.getContext('2d'),
  };
  resizeChart();
}

function resizeChart() {
  if (!chart) return;
  const rect = chart.canvas.parentElement.getBoundingClientRect();
  const dpr = window.devicePixelRatio || 1;
  chart.canvas.width = rect.width * dpr;
  chart.canvas.height = rect.height * dpr;
  chart.ctx.scale(dpr, dpr);
  chart.w = rect.width;
  chart.h = rect.height;
}

function redrawChart() {
  if (!chart || state.samples.length < 2) return;
  resizeChart();

  const ctx = chart.ctx;
  const w = chart.w;
  const h = chart.h;
  const pad = { top: 10, right: 10, bottom: 24, left: 50 };
  const cw = w - pad.left - pad.right;
  const ch = h - pad.top - pad.bottom;

  ctx.clearRect(0, 0, w, h);

  const speeds = state.samples.map(s => s.blockSpeed);
  const maxY = Math.max(...speeds) * 1.1 || 1;
  const minY = 0;

  // Grid lines
  const gridLines = 4;
  ctx.strokeStyle = 'rgba(255,255,255,0.04)';
  ctx.lineWidth = 1;
  ctx.setLineDash([4, 4]);
  for (let i = 0; i <= gridLines; i++) {
    const y = pad.top + (ch / gridLines) * i;
    ctx.beginPath();
    ctx.moveTo(pad.left, y);
    ctx.lineTo(w - pad.right, y);
    ctx.stroke();
  }
  ctx.setLineDash([]);

  // Y-axis labels
  ctx.fillStyle = 'rgba(255,255,255,0.3)';
  ctx.font = '10px Inter, sans-serif';
  ctx.textAlign = 'right';
  ctx.textBaseline = 'middle';
  for (let i = 0; i <= gridLines; i++) {
    const y = pad.top + (ch / gridLines) * i;
    const val = maxY - (maxY / gridLines) * i;
    ctx.fillText(formatSpeedShort(val), pad.left - 6, y);
  }

  // X-axis data size labels
  ctx.textAlign = 'center';
  ctx.textBaseline = 'top';
  const maxBytes = state.samples[state.samples.length - 1].bytesWritten;
  if (state.samples.length > 1) {
    const labelCount = Math.min(5, state.samples.length);
    for (let i = 0; i <= labelCount; i++) {
      const b = (maxBytes / labelCount) * i;
      const x = pad.left + (cw / labelCount) * i;
      ctx.fillText(formatBytesJS(b), x, h - pad.bottom + 6);
    }
  }

  // Area gradient
  const gradient = ctx.createLinearGradient(0, pad.top, 0, pad.top + ch);
  gradient.addColorStop(0, 'rgba(99, 102, 241, 0.3)');
  gradient.addColorStop(0.5, 'rgba(99, 102, 241, 0.08)');
  gradient.addColorStop(1, 'rgba(99, 102, 241, 0)');

  const points = state.samples.map(s => ({
    x: pad.left + (s.bytesWritten / (maxBytes || 1)) * cw,
    y: pad.top + ch - ((s.blockSpeed - minY) / (maxY - minY)) * ch,
  }));

  // Fill area
  ctx.beginPath();
  ctx.moveTo(points[0].x, pad.top + ch);
  points.forEach(p => ctx.lineTo(p.x, p.y));
  ctx.lineTo(points[points.length - 1].x, pad.top + ch);
  ctx.closePath();
  ctx.fillStyle = gradient;
  ctx.fill();

  // Line
  ctx.beginPath();
  ctx.moveTo(points[0].x, points[0].y);
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const cpx = (prev.x + curr.x) / 2;
    ctx.bezierCurveTo(cpx, prev.y, cpx, curr.y, curr.x, curr.y);
  }
  ctx.strokeStyle = '#6366f1';
  ctx.lineWidth = 2;
  ctx.stroke();

  // Glow
  ctx.beginPath();
  ctx.moveTo(points[0].x, points[0].y);
  for (let i = 1; i < points.length; i++) {
    const prev = points[i - 1];
    const curr = points[i];
    const cpx = (prev.x + curr.x) / 2;
    ctx.bezierCurveTo(cpx, prev.y, cpx, curr.y, curr.x, curr.y);
  }
  ctx.strokeStyle = 'rgba(99, 102, 241, 0.4)';
  ctx.lineWidth = 6;
  ctx.stroke();

  // Last point indicator
  if (state.phase === 'running' && points.length > 0) {
    const last = points[points.length - 1];
    ctx.beginPath();
    ctx.arc(last.x, last.y, 4, 0, Math.PI * 2);
    ctx.fillStyle = '#6366f1';
    ctx.fill();
    ctx.beginPath();
    ctx.arc(last.x, last.y, 8, 0, Math.PI * 2);
    ctx.strokeStyle = 'rgba(99, 102, 241, 0.3)';
    ctx.lineWidth = 2;
    ctx.stroke();
  }
}

// ── Wails Events ────────────────────────────────────────────────────────

function setupEvents() {
  if (!runtime) return;

  runtime.EventsOn('bench:started', (data) => {
    state.fileSize = data.fileSize;
    state.fileSizeText = data.fileSizeText;
  });

  runtime.EventsOn('bench:sample', (sample) => {
    state.samples.push(sample);
    state.currentSpeed = sample.blockSpeed;
    state.avgSpeed = sample.avgSpeed;
    state.progress = sample.progress;

    if (sample.blockSpeed > state.maxSpeed) state.maxSpeed = sample.blockSpeed;
    if (sample.blockSpeed < state.minSpeed) state.minSpeed = sample.blockSpeed;

    // Update DOM directly for performance
    updateRunningUI(sample);
    redrawChart();
  });

  runtime.EventsOn('bench:done', (result) => {
    state.result = result;
    state.phase = 'done';
    render();
  });
}

function updateRunningUI(sample) {
  const { value: speedVal, unit: speedUnit } = parseSpeed(sample.blockSpeed);
  const el = (id) => document.getElementById(id);

  const speedNum = el('currentSpeed');
  if (speedNum) speedNum.textContent = speedVal;
  const speedU = el('currentSpeedUnit');
  if (speedU) speedU.textContent = speedUnit;

  const avg = el('statAvg');
  if (avg) avg.textContent = sample.avgText;
  const max = el('statMax');
  if (max) max.textContent = formatSpeedShort(state.maxSpeed);
  const min = el('statMin');
  if (min) min.textContent = state.minSpeed === Infinity ? '—' : formatSpeedShort(state.minSpeed);

  const pFill = el('progressFill');
  if (pFill) pFill.style.width = sample.progress.toFixed(1) + '%';
  const pPct = el('progressPct');
  if (pPct) pPct.textContent = sample.progress.toFixed(1) + '%';
  const pText = el('progressText');
  if (pText) pText.textContent = `${formatBytesJS(sample.bytesWritten)} / ${state.fileSizeText}`;
}

// ── Update Check ────────────────────────────────────────────────────────

async function checkForUpdate() {
  if (!go) return;
  try {
    const ver = await go.GetVersion();
    const badge = document.getElementById('versionBadge');
    if (badge) badge.textContent = ver;

    const info = await go.CheckForUpdate();
    if (info.available) {
      const banner = document.getElementById('updateBanner');
      const link = document.getElementById('updateLink');
      if (banner && link) {
        link.textContent = info.latest;
        link.onclick = (e) => {
          e.preventDefault();
          go.OpenURL(info.url);
        };
        banner.classList.add('show');
      }
    }
  } catch (e) {
    console.error('Update check failed:', e);
  }
}

// ── Helpers ─────────────────────────────────────────────────────────────

function parseSpeed(bps) {
  if (bps >= 1024 * 1024 * 1024) return { value: (bps / (1024 * 1024 * 1024)).toFixed(2), unit: 'GiB/s' };
  if (bps >= 1024 * 1024) return { value: (bps / (1024 * 1024)).toFixed(1), unit: 'MiB/s' };
  if (bps >= 1024) return { value: (bps / 1024).toFixed(0), unit: 'KiB/s' };
  return { value: bps.toFixed(0), unit: 'B/s' };
}

function formatSpeedShort(bps) {
  if (bps >= 1024 * 1024 * 1024) return (bps / (1024 * 1024 * 1024)).toFixed(2) + ' GiB/s';
  if (bps >= 1024 * 1024) return (bps / (1024 * 1024)).toFixed(1) + ' MiB/s';
  if (bps >= 1024) return (bps / 1024).toFixed(0) + ' KiB/s';
  return bps.toFixed(0) + ' B/s';
}

function formatBytesJS(b) {
  if (b >= 1024 ** 4) return (b / 1024 ** 4).toFixed(2) + ' TiB';
  if (b >= 1024 ** 3) return (b / 1024 ** 3).toFixed(2) + ' GiB';
  if (b >= 1024 ** 2) return (b / 1024 ** 2).toFixed(2) + ' MiB';
  if (b >= 1024) return (b / 1024).toFixed(2) + ' KiB';
  return b + ' B';
}

function formatTime(seconds) {
  if (seconds < 60) return Math.round(seconds) + 's';
  const m = Math.floor(seconds / 60);
  const s = Math.round(seconds % 60);
  return m + 'm' + (s > 0 ? s + 's' : '');
}

function escHtml(s) {
  const div = document.createElement('div');
  div.textContent = s || '';
  return div.innerHTML;
}

function showToast(msg) {
  const toast = document.getElementById('toast');
  if (!toast) return;
  toast.textContent = msg;
  toast.classList.add('show');
  setTimeout(() => toast.classList.remove('show'), 3000);
}

// ── Init ────────────────────────────────────────────────────────────────

window.addEventListener('resize', () => {
  if (chart) redrawChart();
});

render();
setupEvents();
checkForUpdate();
