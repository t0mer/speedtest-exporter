/* global fetch, document, setInterval, requestAnimationFrame, performance */
'use strict';

// ── XSS guard ─────────────────────────────────────────────────────────────
const ESC = {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'};
function esc(s) { return String(s == null ? '—' : s).replace(/[&<>"']/g, c => ESC[c]); }
function fmt(n, dec) { return n == null ? '—' : Number(n).toFixed(dec ?? 1); }

// ── Date / time format state (set from settings on load) ──────────────────
let DATE_FORMAT = ''; // "" = browser locale; see formatDate()
let TIME_FORMAT = ''; // "" = browser locale; see formatTime()

// ── Date / time formatters ─────────────────────────────────────────────────
function formatDate(d) {
  if (!DATE_FORMAT) return d.toLocaleDateString();
  const y   = d.getFullYear();
  const mon = String(d.getMonth() + 1).padStart(2, '0');
  const day = String(d.getDate()).padStart(2, '0');
  switch (DATE_FORMAT) {
    case 'YYYY-MM-DD': return `${y}-${mon}-${day}`;
    case 'MM/DD/YYYY': return `${mon}/${day}/${y}`;
    case 'DD/MM/YYYY': return `${day}/${mon}/${y}`;
    case 'DD.MM.YYYY': return `${day}.${mon}.${y}`;
    default:           return d.toLocaleDateString();
  }
}

function formatTime(d) {
  if (!TIME_FORMAT) return d.toLocaleTimeString();
  const h24 = d.getHours();
  const min = String(d.getMinutes()).padStart(2, '0');
  const sec = String(d.getSeconds()).padStart(2, '0');
  switch (TIME_FORMAT) {
    case 'HH:mm':     return `${String(h24).padStart(2, '0')}:${min}`;
    case 'HH:mm:ss':  return `${String(h24).padStart(2, '0')}:${min}:${sec}`;
    case 'hh:mm a': {
      const ampm = h24 >= 12 ? 'PM' : 'AM';
      const h12  = h24 % 12 || 12;
      return `${String(h12).padStart(2, '0')}:${min} ${ampm}`;
    }
    case 'hh:mm:ss a': {
      const ampm = h24 >= 12 ? 'PM' : 'AM';
      const h12  = h24 % 12 || 12;
      return `${String(h12).padStart(2, '0')}:${min}:${sec} ${ampm}`;
    }
    default: return d.toLocaleTimeString();
  }
}

function formatDateTime(isoString) {
  if (!isoString) return '—';
  const d = new Date(isoString);
  if (isNaN(d)) return '—';
  return formatDate(d) + ' ' + formatTime(d);
}

// ── Toast system ──────────────────────────────────────────────────────────
function toast(msg, type = 'ok') {
  const stack = document.getElementById('toast-stack');
  if (!stack) return;
  const el = document.createElement('div');
  el.className = `toast toast-${type}`;
  el.innerHTML = `<span class="toast-icon">${type === 'ok' ? '✓' : '✕'}</span><span>${esc(msg)}</span>`;
  stack.appendChild(el);
  requestAnimationFrame(() => requestAnimationFrame(() => el.classList.add('show')));
  setTimeout(() => {
    el.classList.remove('show');
    setTimeout(() => el.remove(), 250);
  }, 3500);
}

// ── Animated number counter ───────────────────────────────────────────────
function animateNum(el, target, decimals) {
  if (!el) return;
  const from = parseFloat(el.dataset.raw) || 0;
  const dur  = 700;
  const t0   = performance.now();
  el.dataset.raw = target;
  function step(now) {
    const p = Math.min((now - t0) / dur, 1);
    const e = 1 - Math.pow(1 - p, 3); // ease-out cubic
    el.textContent = (from + (target - from) * e).toFixed(decimals);
    if (p < 1) requestAnimationFrame(step);
    else el.textContent = target.toFixed(decimals);
  }
  requestAnimationFrame(step);
  el.classList.add('val-updated');
  setTimeout(() => el.classList.remove('val-updated'), 500);
}

// ── Gauge ─────────────────────────────────────────────────────────────────
// Arc: 8 o'clock (150° SVG) → 4 o'clock (30° SVG), 240° clockwise.
// Start point computed at 150°: (30.72, 140). Max speed for full arc = 1000 Mbps.
const GAUGE_MAX  = 1000;
const GAUGE_DEG  = 240;   // total arc degrees
const GAUGE_START_RAD = 150 * Math.PI / 180;
const GAUGE_R = 80, GAUGE_CX = 100, GAUGE_CY = 100;

function updateGauge(value) {
  const arc = document.getElementById('gauge-arc');
  if (!arc) return;
  const pct = Math.min(Math.max((value || 0) / GAUGE_MAX, 0), 1);
  if (pct === 0) { arc.setAttribute('d', ''); return; }
  const angle    = GAUGE_DEG * pct * Math.PI / 180;
  const endRad   = GAUGE_START_RAD + angle;
  const x1 = GAUGE_CX + GAUGE_R * Math.cos(GAUGE_START_RAD);
  const y1 = GAUGE_CY + GAUGE_R * Math.sin(GAUGE_START_RAD);
  const x2 = GAUGE_CX + GAUGE_R * Math.cos(endRad);
  const y2 = GAUGE_CY + GAUGE_R * Math.sin(endRad);
  const large = angle > Math.PI ? 1 : 0;
  arc.setAttribute('d', `M ${x1.toFixed(2)} ${y1.toFixed(2)} A ${GAUGE_R} ${GAUGE_R} 0 ${large} 1 ${x2.toFixed(2)} ${y2.toFixed(2)}`);
}

// ── Sparkline ─────────────────────────────────────────────────────────────
function renderSparkline(results) {
  const box = document.getElementById('sparkline');
  if (!box) return;
  if (!results || results.length < 2) {
    box.innerHTML = '<div class="no-chart-msg">Not enough data — run at least 2 tests</div>';
    return;
  }
  const data = [...results].reverse(); // oldest first
  const W = 800, H = 110, PX = 24, PY = 12;
  const maxVal = Math.max(...data.flatMap(r => [r.download_mbps, r.upload_mbps]), 1);
  const xS = i => PX + (i / (data.length - 1)) * (W - PX * 2);
  const yS = v => PY + (H - PY * 2) * (1 - v / maxVal);

  const dlPts = data.map((r, i) => `${xS(i).toFixed(1)},${yS(r.download_mbps).toFixed(1)}`);
  const ulPts = data.map((r, i) => `${xS(i).toFixed(1)},${yS(r.upload_mbps).toFixed(1)}`);
  const bX = xS(data.length - 1);
  const bY = H - PY;

  const dlArea = `M ${dlPts.join(' L ')} L ${bX.toFixed(1)},${bY} L ${PX},${bY} Z`;
  const ulArea = `M ${ulPts.join(' L ')} L ${bX.toFixed(1)},${bY} L ${PX},${bY} Z`;

  // Y-axis labels
  const labels = [0, 0.5, 1].map(f => {
    const v = maxVal * f;
    const y = yS(v);
    return `<text x="${PX - 6}" y="${y.toFixed(1)}" text-anchor="end" dominant-baseline="middle" fill="rgba(255,255,255,0.2)" font-size="9" font-family="JetBrains Mono,monospace">${v >= 1 ? Math.round(v) : v.toFixed(1)}</text>`;
  }).join('');

  box.innerHTML = `<svg viewBox="0 0 ${W} ${H}" preserveAspectRatio="none" xmlns="http://www.w3.org/2000/svg">
    <defs>
      <linearGradient id="dlFill" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" stop-color="#22d3ee" stop-opacity="0.25"/>
        <stop offset="100%" stop-color="#22d3ee" stop-opacity="0"/>
      </linearGradient>
      <linearGradient id="ulFill" x1="0" y1="0" x2="0" y2="1">
        <stop offset="0%" stop-color="#a78bfa" stop-opacity="0.15"/>
        <stop offset="100%" stop-color="#a78bfa" stop-opacity="0"/>
      </linearGradient>
    </defs>
    ${[0.25, 0.5, 0.75, 1].map(f => `<line x1="${PX}" y1="${yS(maxVal*f).toFixed(1)}" x2="${W-PX}" y2="${yS(maxVal*f).toFixed(1)}" stroke="rgba(255,255,255,0.04)" stroke-width="1"/>`).join('')}
    ${labels}
    <path d="${ulArea}" fill="url(#ulFill)"/>
    <path d="${dlArea}" fill="url(#dlFill)"/>
    <path d="M ${ulPts.join(' L ')}" fill="none" stroke="#a78bfa" stroke-width="1.5" stroke-linejoin="round" stroke-linecap="round"/>
    <path d="M ${dlPts.join(' L ')}" fill="none" stroke="#22d3ee" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>
    <circle cx="${xS(data.length-1).toFixed(1)}" cy="${yS(data[data.length-1].download_mbps).toFixed(1)}" r="3.5" fill="#22d3ee"/>
    <circle cx="${xS(data.length-1).toFixed(1)}" cy="${yS(data[data.length-1].upload_mbps).toFixed(1)}" r="3" fill="#a78bfa"/>
  </svg>`;
}

// ── Render latest ─────────────────────────────────────────────────────────
function renderLatest(r) {
  const dlEl     = document.getElementById('dl-value');
  const ulEl     = document.getElementById('ul-value');
  const pingEl   = document.getElementById('ping-value');
  const jitterEl = document.getElementById('jitter-value');
  const srvEl    = document.getElementById('server-value');
  const ispEl    = document.getElementById('isp-value');
  const tsEl     = document.getElementById('last-ts');

  if (!r) {
    [dlEl, ulEl, pingEl, jitterEl].forEach(el => { if (el) el.textContent = '—'; });
    if (srvEl)  srvEl.textContent  = '—';
    if (tsEl)   tsEl.textContent   = '';
    updateGauge(0);
    return;
  }

  updateGauge(r.download_mbps);
  if (dlEl)     animateNum(dlEl, r.download_mbps, 1);
  if (ulEl)     animateNum(ulEl, r.upload_mbps, 1);
  if (pingEl)   animateNum(pingEl, r.ping_ms, 1);
  if (jitterEl) animateNum(jitterEl, r.jitter_ms, 1);
  if (srvEl)    srvEl.textContent = r.server_name || '—';
  if (ispEl)    ispEl.textContent = r.isp || '';
  if (tsEl) {
    const d = new Date(r.timestamp);
    tsEl.textContent = `Last test: ${formatDate(d)} · ${formatTime(d)}`;
  }
}

// ── Render summary ────────────────────────────────────────────────────────
function renderSummary(s) {
  const grid = document.getElementById('summary-grid');
  if (!grid) return;
  if (!s || s.count === 0) {
    grid.innerHTML = '<p class="no-data-msg">No data for the last 7 days.</p>';
    return;
  }
  const cards = [
    { val: fmt(s.avg_download_mbps), unit: 'Mbps avg', lbl: 'Avg Download' },
    { val: fmt(s.avg_upload_mbps),   unit: 'Mbps avg', lbl: 'Avg Upload' },
    { val: fmt(s.avg_ping_ms),       unit: 'ms avg',   lbl: 'Avg Ping' },
    { val: s.count,                  unit: 'tests',    lbl: `Last ${s.days} Days` },
    { val: fmt(s.min_download_mbps) + ' – ' + fmt(s.max_download_mbps), unit: 'Mbps', lbl: 'DL Range' },
  ];
  grid.innerHTML = cards.map(c => `<div class="summ-card">
    <div class="summ-val">${esc(String(c.val))}</div>
    <div class="summ-unit">${esc(c.unit)}</div>
    <div class="summ-lbl">${esc(c.lbl)}</div>
  </div>`).join('');
}

// ── Results table with pagination + delta arrows ──────────────────────────
const PAGE_SIZE = 5;
let currentPage = 0;

// delta returns an ▲/▼ span; higherIsBetter=true for speed, false for latency.
function delta(curr, prev, higherIsBetter) {
  if (prev == null || curr == null) return '';
  const diff = curr - prev;
  // Ignore changes smaller than 2% of the previous value.
  if (prev !== 0 && Math.abs(diff / prev) < 0.02) return '';
  const up   = diff > 0;
  const good = up === higherIsBetter;
  return `<span class="delta ${up ? 'up' : 'dn'} ${good ? 'good' : 'bad'}">${up ? '▲' : '▼'}</span>`;
}

function renderTablePage(rows, compRow) {
  const tbody = document.getElementById('results-tbody');
  if (!tbody) return;
  if (!rows || rows.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" class="empty-row">No results yet. Run a test to get started.</td></tr>';
    return;
  }
  const srcClass = s => ({ manual: 'src-manual', scheduled: 'src-scheduled', api: 'src-api' }[s] || '');
  tbody.innerHTML = rows.map((r, i) => {
    // "previous" = the next item in time (older), which is rows[i+1] or compRow for the last row.
    const prev = rows[i + 1] || compRow || null;
    const dl   = delta(r.download_mbps, prev?.download_mbps, true);
    const ul   = delta(r.upload_mbps,   prev?.upload_mbps,   true);
    const pg   = delta(r.ping_ms,       prev?.ping_ms,       false);
    const jt   = delta(r.jitter_ms,     prev?.jitter_ms,     false);
    return `<tr>
      <td>${esc(formatDateTime(r.timestamp))}</td>
      <td class="num-cell">${fmt(r.download_mbps)}${dl} <span class="unit">Mbps</span></td>
      <td class="num-cell">${fmt(r.upload_mbps)}${ul} <span class="unit">Mbps</span></td>
      <td class="num-cell">${fmt(r.ping_ms)}${pg} <span class="unit">ms</span></td>
      <td class="num-cell">${fmt(r.jitter_ms)}${jt} <span class="unit">ms</span></td>
      <td>${esc(r.server_name)}</td>
      <td><span class="src-badge ${srcClass(r.source)}">${esc(r.source)}</span></td>
    </tr>`;
  }).join('');
}

function updatePagination(page, hasNext, totalOnPage) {
  const prevBtn  = document.getElementById('page-prev');
  const nextBtn  = document.getElementById('page-next');
  const pageNum  = document.getElementById('page-num');
  const pagInfo  = document.getElementById('pag-info');
  if (prevBtn) prevBtn.disabled = page === 0;
  if (nextBtn) nextBtn.disabled = !hasNext;
  if (pageNum) pageNum.textContent = `Page ${page + 1}`;
  if (pagInfo && totalOnPage > 0) {
    const from = page * PAGE_SIZE + 1;
    const to   = from + totalOnPage - 1;
    pagInfo.textContent = `${from}–${to}`;
  } else if (pagInfo) {
    pagInfo.textContent = '';
  }
}

async function loadPage(page) {
  const offset  = page * PAGE_SIZE;
  // Fetch one extra so we know if there's a next page AND have a comparison row for the last entry.
  const results = await fetch(`/api/results?limit=${PAGE_SIZE + 1}&offset=${offset}`)
    .then(r => r.json()).catch(() => []);

  const hasNext  = results.length > PAGE_SIZE;
  const pageRows = results.slice(0, PAGE_SIZE);
  const compRow  = results[PAGE_SIZE] || null; // older item for last-row delta

  renderTablePage(pageRows, compRow);
  updatePagination(page, hasNext, pageRows.length);
  currentPage = page;
}

// ── Load all dashboard data ────────────────────────────────────────────────
async function loadAll(resetPage) {
  if (resetPage) currentPage = 0;
  const [latestRes, summaryRes, sparkRes] = await Promise.allSettled([
    fetch('/api/results/latest').then(r => r.ok ? r.json() : null),
    fetch('/api/summary?days=7').then(r => r.json()),
    fetch('/api/results?limit=25').then(r => r.json()),
  ]);
  renderLatest(latestRes.status === 'fulfilled' ? latestRes.value : null);
  renderSummary(summaryRes.status === 'fulfilled' ? summaryRes.value : null);
  renderSparkline(sparkRes.status === 'fulfilled' ? sparkRes.value : []);
  await loadPage(currentPage);
}

// ── Live progress helpers ─────────────────────────────────────────────────
const PHASE_LABELS = {
  connecting: 'CONNECTING',
  ping:       'PING',
  download:   'DOWNLOAD',
  upload:     'UPLOAD',
  done:       'DONE',
  error:      'ERROR',
};

function setTestPhase(phase) {
  const lbl = document.getElementById('phase-label');
  if (lbl) lbl.textContent = PHASE_LABELS[phase] || phase.toUpperCase();
}

function setTestRunning(running) {
  const card = document.getElementById('gauge-card');
  if (card) card.classList.toggle('testing', running);
}

function handleProgressEvent(ev) {
  setTestPhase(ev.phase);

  if (ev.phase === 'connecting' && ev.server_name) {
    const srvEl = document.getElementById('server-value');
    if (srvEl) { srvEl.textContent = ev.server_name; srvEl.dataset.raw = '0'; }
  }

  if (ev.phase === 'ping' && ev.ping_ms > 0) {
    const el = document.getElementById('ping-value');
    if (el) { el.textContent = ev.ping_ms.toFixed(1); el.dataset.raw = ev.ping_ms; }
  }

  if (ev.phase === 'download' && ev.download_mbps > 0) {
    const el = document.getElementById('dl-value');
    if (el) { el.textContent = ev.download_mbps.toFixed(1); el.dataset.raw = ev.download_mbps; }
    updateGauge(ev.download_mbps);
  }

  if (ev.phase === 'upload' && ev.upload_mbps > 0) {
    const el = document.getElementById('ul-value');
    if (el) { el.textContent = ev.upload_mbps.toFixed(1); el.dataset.raw = ev.upload_mbps; }
  }

  if (ev.phase === 'done') {
    // Animate the final values for a polished landing.
    if (ev.download_mbps) { animateNum(document.getElementById('dl-value'), ev.download_mbps, 1); updateGauge(ev.download_mbps); }
    if (ev.upload_mbps)   { animateNum(document.getElementById('ul-value'), ev.upload_mbps, 1); }
    if (ev.ping_ms)       { animateNum(document.getElementById('ping-value'), ev.ping_ms, 1); }
    if (ev.server_name)   { const s = document.getElementById('server-value'); if (s) s.textContent = ev.server_name; }
  }
}

// ── Run test buttons ──────────────────────────────────────────────────────
async function runTest() {
  const btn    = document.getElementById('run-btn');
  const mobBtn = document.getElementById('mob-run-btn');
  const label  = btn ? btn.querySelector('.run-label') : null;
  if (btn) btn.disabled = true;
  if (mobBtn) mobBtn.disabled = true;
  if (label) label.textContent = 'Running…';
  setTestRunning(true);
  setTestPhase('connecting');

  try {
    const res = await fetch('/api/test/stream', { method: 'POST' });
    if (!res.ok) { throw new Error(await res.text()); }

    const reader  = res.body.getReader();
    const decoder = new TextDecoder();
    let   buffer  = '';

    while (true) {
      const { done, value } = await reader.read();
      if (done) break;
      buffer += decoder.decode(value, { stream: true });
      const lines = buffer.split('\n');
      buffer = lines.pop() ?? '';
      for (const line of lines) {
        if (!line.startsWith('data: ')) continue;
        try {
          const ev = JSON.parse(line.slice(6));
          handleProgressEvent(ev);
          if (ev.phase === 'error') toast('Test error: ' + (ev.error || 'unknown'), 'err');
        } catch { /* ignore malformed event */ }
      }
    }

    await loadAll(true); // reset to page 1 and refresh
    toast('Test complete', 'ok');
  } catch (e) {
    toast('Test failed: ' + e.message, 'err');
  } finally {
    if (btn) btn.disabled = false;
    if (mobBtn) mobBtn.disabled = false;
    if (label) label.textContent = 'Run Test';
    setTestRunning(false);
  }
}
document.getElementById('run-btn')?.addEventListener('click', runTest);
document.getElementById('mob-run-btn')?.addEventListener('click', runTest);

// ── Auto-refresh countdown ────────────────────────────────────────────────
let countdown = 60;
setInterval(() => {
  const el = document.getElementById('refresh-timer');
  if (el) el.textContent = countdown + 's';
  countdown--;
  if (countdown < 0) { countdown = 60; loadAll(); }
}, 1000);

// ── Tab switching ─────────────────────────────────────────────────────────
function switchTab(tab) {
  document.querySelectorAll('.nav-pill, .mob-tab').forEach(b => {
    b.classList.toggle('active', b.dataset.tab === tab);
  });
  document.querySelectorAll('.tab-panel').forEach(p => {
    p.classList.toggle('active', p.id === 'tab-' + tab);
  });
  if (tab === 'settings')      loadSettings();
  if (tab === 'notifications') loadChannels();
}
document.querySelectorAll('[data-tab]').forEach(btn => {
  if (!btn.dataset.tab) return;
  btn.addEventListener('click', () => switchTab(btn.dataset.tab));
});

// ═══════════════════════════════════════════════════════════
// SETTINGS
// ═══════════════════════════════════════════════════════════
// ── Preferred-server display helpers ─────────────────────────────────────
function setPreferredServerDisplay(id, name) {
  const label  = document.getElementById('pref-server-label');
  const chip   = document.getElementById('pref-server-id-chip');
  const clearB = document.getElementById('clear-server-btn');
  const hidId  = document.getElementById('pref-server-id');
  const hidNm  = document.getElementById('pref-server-name');
  if (label)  label.textContent  = id ? esc(name || id) : 'Nearest available';
  if (chip)  { chip.textContent = id ? '#' + id : ''; chip.style.display = id ? '' : 'none'; }
  if (clearB)  clearB.style.display = id ? '' : 'none';
  if (hidId)   hidId.value = id || '';
  if (hidNm)   hidNm.value = name || '';
}

// Show/hide preferred server field based on engine
function updatePreferredServerVisibility() {
  const engine = document.getElementById('cfg-engine')?.value;
  const field  = document.getElementById('preferred-server-field');
  if (field) field.classList.toggle('pref-server-hidden', engine === 'ookla');
}
document.getElementById('cfg-engine')?.addEventListener('change', updatePreferredServerVisibility);

// ── Settings load / save ──────────────────────────────────────────────────
async function loadSettings() {
  try {
    const s = await fetch('/api/settings').then(r => r.json());
    const g = id => document.getElementById(id);
    const v = (id, val) => { const el = g(id); if (el) el.value = val || ''; };
    const n = (id, val) => { const el = g(id); if (el) el.value = val || 0; };
    v('cfg-engine',           s.engine || 'go');
    v('cfg-schedule',         s.schedule || '');
    n('cfg-min-download',     s.min_download_mbps);
    n('cfg-min-upload',       s.min_upload_mbps);
    n('cfg-max-ping',         s.max_ping_ms);
    n('cfg-max-jitter',       s.max_jitter_ms);
    n('cfg-max-packet-loss',  s.max_packet_loss_ratio);
    n('cfg-cooldown',         s.cooldown_minutes);
    v('cfg-webhooks', (s.webhooks || []).join('\n'));
    DATE_FORMAT = s.date_format || '';
    TIME_FORMAT = s.time_format || '';
    v('cfg-date-format', DATE_FORMAT);
    v('cfg-time-format', TIME_FORMAT);
    v('cfg-export-passphrase', s.export_passphrase || '');
    setPreferredServerDisplay(s.preferred_server_id || '', s.preferred_server_name || '');
    updatePreferredServerVisibility();
  } catch { /* already loaded */ }
}

async function saveSettings() {
  const btn = document.getElementById('save-btn');
  const msg = document.getElementById('settings-msg');
  if (!btn || !msg) return;
  btn.disabled = true;
  msg.textContent = '';
  msg.className = 'save-msg';
  const g = id => document.getElementById(id);
  const payload = {
    engine:                g('cfg-engine')?.value,
    schedule:              g('cfg-schedule')?.value?.trim() || '',
    min_download_mbps:     parseFloat(g('cfg-min-download')?.value) || 0,
    min_upload_mbps:       parseFloat(g('cfg-min-upload')?.value) || 0,
    max_ping_ms:           parseFloat(g('cfg-max-ping')?.value) || 0,
    max_jitter_ms:         parseFloat(g('cfg-max-jitter')?.value) || 0,
    max_packet_loss_ratio: parseFloat(g('cfg-max-packet-loss')?.value) || 0,
    cooldown_minutes:      parseInt(g('cfg-cooldown')?.value) || 0,
    webhooks: (g('cfg-webhooks')?.value || '').split('\n').map(u => u.trim()).filter(Boolean),
    preferred_server_id:   g('pref-server-id')?.value || '',
    preferred_server_name: g('pref-server-name')?.value || '',
    date_format:           g('cfg-date-format')?.value || '',
    time_format:           g('cfg-time-format')?.value || '',
    export_passphrase:     g('cfg-export-passphrase')?.value || '',
  };
  try {
    const res  = await fetch('/api/settings', { method: 'PUT', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
    const data = await res.json();
    if (!res.ok) {
      msg.textContent = data.error || 'Save failed';
      msg.className = 'save-msg err';
      toast(data.error || 'Save failed', 'err');
    } else {
      msg.textContent = 'Saved';
      msg.className = 'save-msg ok';
      toast('Settings saved', 'ok');
      DATE_FORMAT = g('cfg-date-format')?.value || '';
      TIME_FORMAT = g('cfg-time-format')?.value || '';
      setTimeout(() => { msg.textContent = ''; }, 3000);
    }
  } catch (e) {
    msg.textContent = 'Network error';
    msg.className = 'save-msg err';
    toast('Network error: ' + e.message, 'err');
  } finally {
    btn.disabled = false;
  }
}
document.getElementById('save-btn')?.addEventListener('click', saveSettings);

// ── Export / Import ───────────────────────────────────────────────────────
async function exportSettings(encrypted) {
  try {
    const res = await fetch(`/api/settings/export?encrypted=${encrypted}`);
    if (!res.ok) {
      const d = await res.json().catch(() => ({}));
      toast(d.error || 'Export failed', 'err');
      return;
    }
    const blob = await res.blob();
    const a = document.createElement('a');
    a.href = URL.createObjectURL(blob);
    a.download = 'speedtest-settings.json';
    document.body.appendChild(a);
    a.click();
    document.body.removeChild(a);
    URL.revokeObjectURL(a.href);
    toast('Settings exported', 'ok');
  } catch (e) {
    toast('Export failed: ' + e.message, 'err');
  }
}

async function importSettings() {
  const file = document.getElementById('import-file')?.files?.[0];
  if (!file) return;
  const msg = document.getElementById('import-msg');
  if (msg) { msg.textContent = 'Importing…'; msg.className = 'save-msg'; }
  try {
    const res = await fetch('/api/settings/import', {
      method: 'POST',
      headers: { 'Content-Type': 'application/json' },
      body: await file.text(),
    });
    const data = await res.json();
    if (!res.ok) {
      if (msg) { msg.textContent = data.error || 'Import failed'; msg.className = 'save-msg err'; }
      toast(data.error || 'Import failed', 'err');
      return;
    }
    const n = data.channels_imported ?? 0;
    if (msg) { msg.textContent = `Imported (${n} channel${n !== 1 ? 's' : ''})`; msg.className = 'save-msg ok'; }
    toast(`Settings imported — ${n} channel${n !== 1 ? 's' : ''} restored`, 'ok');
    await loadSettings();
    await loadChannels();
    setTimeout(() => { if (msg) msg.textContent = ''; }, 4000);
  } catch (e) {
    if (msg) { msg.textContent = 'Network error'; msg.className = 'save-msg err'; }
    toast('Network error: ' + e.message, 'err');
  }
}

// ── Server picker ─────────────────────────────────────────────────────────
let allServers = null; // cached after first fetch

async function openServerPicker() {
  document.getElementById('server-picker-dialog').style.display = '';
  document.getElementById('server-search').value = '';
  renderServerList(null); // show loading
  try {
    if (!allServers) {
      allServers = await fetch('/api/servers').then(r => r.json());
    }
    renderServerList(allServers);
  } catch (e) {
    const list = document.getElementById('servers-list');
    if (list) list.innerHTML = `<div class="servers-loading" style="color:var(--red)">Failed to load servers: ${esc(e.message)}</div>`;
  }
}

function renderServerList(servers) {
  const list = document.getElementById('servers-list');
  if (!list) return;
  if (!servers) {
    list.innerHTML = '<div class="servers-loading"><svg width="18" height="18" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2" class="spin"><path d="M21 12a9 9 0 1 1-6.219-8.56"/></svg> Fetching nearby servers…</div>';
    return;
  }
  if (servers.length === 0) {
    list.innerHTML = '<div class="servers-loading">No servers found.</div>';
    return;
  }
  const currentId = document.getElementById('pref-server-id')?.value || '';
  const q = (document.getElementById('server-search')?.value || '').toLowerCase();
  const filtered = q
    ? servers.filter(s =>
        s.name.toLowerCase().includes(q) ||
        s.country.toLowerCase().includes(q) ||
        s.sponsor.toLowerCase().includes(q) ||
        s.id.includes(q))
    : servers;

  if (filtered.length === 0) {
    list.innerHTML = '<div class="servers-loading">No servers match your search.</div>';
    return;
  }

  list.innerHTML = filtered.map(s => {
    const dist = s.distance_km > 0 ? `${s.distance_km.toFixed(0)} km` : '';
    const sel  = s.id === currentId ? ' selected' : '';
    return `<div class="server-row${sel}" data-id="${esc(s.id)}" data-name="${esc(s.name + ', ' + s.country)}">
      <div class="server-row-info">
        <div class="server-row-name">${esc(s.name)}, ${esc(s.country)}</div>
        <div class="server-row-sponsor">${esc(s.sponsor)}</div>
      </div>
      <div class="server-row-meta">
        <span class="server-row-id">#${esc(s.id)}</span>
        ${dist ? `<span class="server-row-dist">${esc(dist)}</span>` : ''}
      </div>
    </div>`;
  }).join('');

  list.querySelectorAll('.server-row').forEach(row => {
    row.addEventListener('click', () => {
      setPreferredServerDisplay(row.dataset.id, row.dataset.name);
      document.getElementById('server-picker-dialog').style.display = 'none';
    });
  });
}

function closeServerPicker() {
  document.getElementById('server-picker-dialog').style.display = 'none';
}

document.getElementById('browse-servers-btn')?.addEventListener('click', openServerPicker);
document.getElementById('clear-server-btn')?.addEventListener('click', () => setPreferredServerDisplay('', ''));
document.getElementById('close-server-picker')?.addEventListener('click', closeServerPicker);
document.getElementById('cancel-server-picker')?.addEventListener('click', closeServerPicker);
document.getElementById('server-picker-dialog')?.addEventListener('click', function(e) {
  if (e.target === this) closeServerPicker();
});
document.getElementById('server-search')?.addEventListener('input', () => {
  if (allServers) renderServerList(allServers);
});

// ═══════════════════════════════════════════════════════════
// NOTIFICATIONS
// ═══════════════════════════════════════════════════════════
let editingChannelId = null;

async function loadChannels() {
  const list = document.getElementById('channels-list');
  if (!list) return;
  try {
    const channels = await fetch('/api/notifications').then(r => r.json());
    if (!channels || channels.length === 0) {
      list.innerHTML = '<div class="no-channels">No channels configured yet. Click "+ Add Channel" to get started.</div>';
      return;
    }
    list.innerHTML = channels.map(ch => `<div class="ch-row" data-id="${ch.id}" data-name="${esc(ch.name)}">
      <div class="ch-info">
        <div class="ch-name">${esc(ch.name)}</div>
        <span class="ch-prov-badge">${esc(ch.provider)}</span>
      </div>
      <div class="ch-status ${ch.enabled ? 'on' : 'off'}">
        <svg width="8" height="8" viewBox="0 0 24 24" fill="currentColor"><circle cx="12" cy="12" r="8"/></svg>
        ${ch.enabled ? 'On' : 'Off'}
      </div>
      <div class="ch-actions">
        <button class="btn-secondary ch-edit" title="Edit">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><path d="M11 4H4a2 2 0 0 0-2 2v14a2 2 0 0 0 2 2h14a2 2 0 0 0 2-2v-7"/><path d="M18.5 2.5a2.121 2.121 0 0 1 3 3L12 15l-4 1 1-4 9.5-9.5z"/></svg>
          Edit
        </button>
        <button class="btn-danger ch-del" title="Delete">
          <svg width="12" height="12" viewBox="0 0 24 24" fill="none" stroke="currentColor" stroke-width="2"><polyline points="3,6 5,6 21,6"/><path d="M19 6l-1 14a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2L5 6"/><path d="M10 11v6M14 11v6"/><path d="M9 6V4a1 1 0 0 1 1-1h4a1 1 0 0 1 1 1v2"/></svg>
          Delete
        </button>
      </div>
    </div>`).join('');
    list.querySelectorAll('.ch-edit').forEach(btn => {
      const row = btn.closest('.ch-row');
      btn.addEventListener('click', () => openEditDialog(Number(row.dataset.id)));
    });
    list.querySelectorAll('.ch-del').forEach(btn => {
      const row = btn.closest('.ch-row');
      btn.addEventListener('click', () => deleteChannel(Number(row.dataset.id), row.dataset.name));
    });
  } catch (e) {
    list.innerHTML = '<div class="no-channels">Failed to load channels.</div>';
  }
}

function switchProviderFields(provider) {
  document.querySelectorAll('.provider-fields').forEach(el => el.style.display = 'none');
  const el = document.getElementById('fields-' + provider);
  if (el) el.style.display = '';
}

function openAddDialog() {
  editingChannelId = null;
  document.getElementById('dialog-title').textContent = 'Add Channel';
  document.getElementById('ch-name').value = '';
  document.getElementById('ch-provider').value = 'shoutrrr';
  switchProviderFields('shoutrrr');
  ['ch-shoutrrr-url','ch-ga-instance','ch-ga-token','ch-ga-phone','ch-ga-apiurl',
   'ch-wa-baseurl','ch-wa-phone','ch-wa-user','ch-wa-pass'].forEach(id => {
    const el = document.getElementById(id); if (el) el.value = '';
  });
  document.getElementById('ch-enabled').checked    = true;
  document.getElementById('ch-on-success').checked = true;
  document.getElementById('ch-on-failure').checked = true;
  document.getElementById('ch-test-msg').textContent = '';
  document.getElementById('channel-dialog').style.display = '';
}

async function openEditDialog(id) {
  try {
    const channels = await fetch('/api/notifications').then(r => r.json());
    const ch = channels.find(c => c.id === id);
    if (!ch) return;
    editingChannelId = id;
    document.getElementById('dialog-title').textContent = 'Edit Channel';
    document.getElementById('ch-name').value = ch.name;
    document.getElementById('ch-provider').value = ch.provider;
    switchProviderFields(ch.provider);
    setProviderConfig(ch.provider, ch.config);
    document.getElementById('ch-enabled').checked    = ch.enabled;
    document.getElementById('ch-on-success').checked = ch.notify_on_success;
    document.getElementById('ch-on-failure').checked = ch.notify_on_failure;
    document.getElementById('ch-test-msg').textContent = '';
    document.getElementById('channel-dialog').style.display = '';
  } catch {}
}

function getProviderConfig() {
  const p = document.getElementById('ch-provider').value;
  if (p === 'shoutrrr') return { url: document.getElementById('ch-shoutrrr-url').value.trim() };
  if (p === 'greenapi') return {
    instance_id: document.getElementById('ch-ga-instance').value.trim(),
    token:       document.getElementById('ch-ga-token').value,
    phone:       document.getElementById('ch-ga-phone').value.trim(),
    api_url:     document.getElementById('ch-ga-apiurl').value.trim(),
  };
  return {
    base_url: document.getElementById('ch-wa-baseurl').value.trim(),
    phone:    document.getElementById('ch-wa-phone').value.trim(),
    username: document.getElementById('ch-wa-user').value.trim(),
    password: document.getElementById('ch-wa-pass').value,
  };
}

function setProviderConfig(provider, config) {
  if (!config) return;
  if (provider === 'shoutrrr') {
    document.getElementById('ch-shoutrrr-url').value = config.url || '';
  } else if (provider === 'greenapi') {
    document.getElementById('ch-ga-instance').value = config.instance_id || '';
    document.getElementById('ch-ga-token').value    = config.token || '';
    document.getElementById('ch-ga-phone').value    = config.phone || '';
    document.getElementById('ch-ga-apiurl').value   = config.api_url || '';
  } else if (provider === 'whatsapp_web') {
    document.getElementById('ch-wa-baseurl').value  = config.base_url || '';
    document.getElementById('ch-wa-phone').value    = config.phone || '';
    document.getElementById('ch-wa-user').value     = config.username || '';
    document.getElementById('ch-wa-pass').value     = config.password || '';
  }
}

async function saveChannel() {
  const provider = document.getElementById('ch-provider').value;
  const name = document.getElementById('ch-name').value.trim();
  if (!name) { toast('Name is required', 'err'); return; }
  const payload = {
    name, provider,
    config:            getProviderConfig(),
    enabled:           document.getElementById('ch-enabled').checked,
    notify_on_success: document.getElementById('ch-on-success').checked,
    notify_on_failure: document.getElementById('ch-on-failure').checked,
  };
  const url    = editingChannelId ? `/api/notifications/${editingChannelId}` : '/api/notifications';
  const method = editingChannelId ? 'PUT' : 'POST';
  try {
    const res  = await fetch(url, { method, headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
    const data = await res.json();
    if (!res.ok) { toast(data.error || 'Save failed', 'err'); return; }
    document.getElementById('channel-dialog').style.display = 'none';
    await loadChannels();
    toast(editingChannelId ? 'Channel updated' : 'Channel added', 'ok');
  } catch (e) { toast('Network error: ' + e.message, 'err'); }
}

async function deleteChannel(id, name) {
  if (!confirm(`Delete channel "${name}"?`)) return;
  try {
    const res = await fetch(`/api/notifications/${id}`, { method: 'DELETE' });
    if (res.status !== 204 && !res.ok) { toast('Delete failed', 'err'); return; }
    await loadChannels();
    toast('Channel deleted', 'ok');
  } catch (e) { toast('Network error: ' + e.message, 'err'); }
}

async function sendTestNotification() {
  const provider = document.getElementById('ch-provider').value;
  const msgEl    = document.getElementById('ch-test-msg');
  msgEl.textContent = 'Sending…';
  const payload = editingChannelId
    ? { id: editingChannelId }
    : { provider, config: getProviderConfig() };
  try {
    const res  = await fetch('/api/notifications/test', { method: 'POST', headers: { 'Content-Type': 'application/json' }, body: JSON.stringify(payload) });
    const data = await res.json();
    msgEl.textContent = res.ok ? '✓ Sent!' : '✕ ' + (data.error || 'Failed');
    msgEl.style.color = res.ok ? 'var(--green)' : 'var(--red)';
  } catch (e) {
    msgEl.textContent = '✕ ' + e.message;
    msgEl.style.color = 'var(--red)';
  }
}

function closeDialog() {
  document.getElementById('channel-dialog').style.display = 'none';
}

document.getElementById('add-channel-btn')?.addEventListener('click', openAddDialog);
document.getElementById('ch-save-btn')?.addEventListener('click', saveChannel);
document.getElementById('ch-cancel-btn')?.addEventListener('click', closeDialog);
document.getElementById('ch-cancel-btn2')?.addEventListener('click', closeDialog);
document.getElementById('ch-test-btn')?.addEventListener('click', sendTestNotification);
document.getElementById('ch-provider')?.addEventListener('change', e => switchProviderFields(e.target.value));

// Close dialog on overlay click
document.getElementById('channel-dialog')?.addEventListener('click', function(e) {
  if (e.target === this) closeDialog();
});

// ── Export/Import listeners ───────────────────────────────────────────────
document.getElementById('export-enc-btn')?.addEventListener('click', () => exportSettings(true));
document.getElementById('export-plain-btn')?.addEventListener('click', () => exportSettings(false));
document.getElementById('import-btn')?.addEventListener('click', () => {
  const fi = document.getElementById('import-file');
  if (fi) { fi.value = ''; fi.click(); }
});
document.getElementById('import-file')?.addEventListener('change', importSettings);

// ── Pagination listeners ──────────────────────────────────────────────────
document.getElementById('page-prev')?.addEventListener('click', () => {
  if (currentPage > 0) loadPage(currentPage - 1);
});
document.getElementById('page-next')?.addEventListener('click', () => {
  loadPage(currentPage + 1);
});

// ── Boot ──────────────────────────────────────────────────────────────────
loadSettings().then(() => loadAll());
