/* global fetch, document, setInterval */

// esc() prevents XSS: escape every API-sourced string before inserting into innerHTML.
const ESC = {'&':'&amp;','<':'&lt;','>':'&gt;','"':'&quot;',"'":'&#39;'};
function esc(s) { return String(s == null ? '—' : s).replace(/[&<>"']/g, c => ESC[c]); }

async function loadAll() {
  const [latestRes, summaryRes, resultsRes] = await Promise.allSettled([
    fetch('/api/results/latest').then(r => r.ok ? r.json() : null),
    fetch('/api/summary?days=7').then(r => r.json()),
    fetch('/api/results?limit=25').then(r => r.json()),
  ]);

  renderLatest(latestRes.status === 'fulfilled' ? latestRes.value : null);
  renderSummary(summaryRes.status === 'fulfilled' ? summaryRes.value : null);
  renderTable(resultsRes.status === 'fulfilled' ? resultsRes.value : []);
}

function fmt(n, dec) { return n == null ? '—' : Number(n).toFixed(dec ?? 1); }

function renderLatest(r) {
  const el = document.getElementById('latest-cards');
  if (!r) { el.innerHTML = '<p class="no-data">No results yet. Run a test to get started.</p>'; return; }
  el.innerHTML = [
    card(fmt(r.download_mbps), 'Mbps', 'Download'),
    card(fmt(r.upload_mbps), 'Mbps', 'Upload'),
    card(fmt(r.ping_ms), 'ms', 'Ping'),
    card(fmt(r.jitter_ms), 'ms', 'Jitter'),
    card(esc(r.server_name), '', 'Server'),
    card(new Date(r.timestamp).toLocaleTimeString(), new Date(r.timestamp).toLocaleDateString(), 'Time'),
  ].join('');
}

function renderSummary(s) {
  const el = document.getElementById('summary-cards');
  if (!s || s.count === 0) { el.innerHTML = '<p class="no-data">No data for the last 7 days.</p>'; return; }
  el.innerHTML = [
    card(fmt(s.avg_download_mbps), 'Mbps avg', 'Download'),
    card(fmt(s.avg_upload_mbps), 'Mbps avg', 'Upload'),
    card(fmt(s.avg_ping_ms), 'ms avg', 'Ping'),
    card(s.count, 'tests', `Last ${s.days} Days`),
    card(fmt(s.min_download_mbps) + ' – ' + fmt(s.max_download_mbps), 'Mbps', 'DL Range'),
  ].join('');
}

function renderTable(results) {
  const tbody = document.querySelector('#results-table tbody');
  if (!results || results.length === 0) {
    tbody.innerHTML = '<tr><td colspan="7" class="no-data">No results yet.</td></tr>';
    return;
  }
  tbody.innerHTML = results.map(r => `<tr>
    <td>${esc(new Date(r.timestamp).toLocaleString())}</td>
    <td>${fmt(r.download_mbps)} Mbps</td>
    <td>${fmt(r.upload_mbps)} Mbps</td>
    <td>${fmt(r.ping_ms)} ms</td>
    <td>${fmt(r.jitter_ms)} ms</td>
    <td>${esc(r.server_name)}</td>
    <td>${esc(r.source)}</td>
  </tr>`).join('');
}

function card(value, unit, label) {
  return `<div class="card">
    <div class="card-value">${value}</div>
    <div class="card-unit">${unit}</div>
    <div class="card-label">${label}</div>
  </div>`;
}

document.getElementById('run-btn').addEventListener('click', async () => {
  const btn = document.getElementById('run-btn');
  btn.disabled = true;
  btn.textContent = 'Running…';
  try {
    const res = await fetch('/api/test', { method: 'POST' });
    if (!res.ok) throw new Error(await res.text());
    await loadAll();
  } catch (e) {
    alert('Test failed: ' + e.message);
  } finally {
    btn.disabled = false;
    btn.textContent = 'Run Test Now';
  }
});

let countdown = 60;
setInterval(() => {
  const el = document.getElementById('refresh-timer');
  if (el) el.textContent = `Auto-refreshing in ${countdown}s`;
  countdown--;
  if (countdown < 0) { countdown = 60; loadAll(); }
}, 1000);

loadAll();

// ── Settings tab ──────────────────────────────────────────────────────────

async function loadSettings() {
  const s = await fetch('/api/settings').then(r => r.json());
  document.getElementById('cfg-engine').value = s.engine || 'go';
  document.getElementById('cfg-schedule').value = s.schedule || '';
  document.getElementById('cfg-min-download').value = s.min_download_mbps || 0;
  document.getElementById('cfg-min-upload').value = s.min_upload_mbps || 0;
  document.getElementById('cfg-max-ping').value = s.max_ping_ms || 0;
  document.getElementById('cfg-max-jitter').value = s.max_jitter_ms || 0;
  document.getElementById('cfg-max-packet-loss').value = s.max_packet_loss_ratio || 0;
  document.getElementById('cfg-cooldown').value = s.cooldown_minutes || 0;
  document.getElementById('cfg-webhooks').value = (s.webhooks || []).join('\n');
}

async function saveSettings() {
  const btn = document.getElementById('save-btn');
  const msg = document.getElementById('settings-msg');
  btn.disabled = true;
  msg.textContent = '';
  msg.className = '';

  const webhooksRaw = document.getElementById('cfg-webhooks').value;
  const webhooks = webhooksRaw.split('\n').map(u => u.trim()).filter(u => u.length > 0);

  const payload = {
    engine:               document.getElementById('cfg-engine').value,
    schedule:             document.getElementById('cfg-schedule').value.trim(),
    min_download_mbps:    parseFloat(document.getElementById('cfg-min-download').value) || 0,
    min_upload_mbps:      parseFloat(document.getElementById('cfg-min-upload').value) || 0,
    max_ping_ms:          parseFloat(document.getElementById('cfg-max-ping').value) || 0,
    max_jitter_ms:        parseFloat(document.getElementById('cfg-max-jitter').value) || 0,
    max_packet_loss_ratio:parseFloat(document.getElementById('cfg-max-packet-loss').value) || 0,
    cooldown_minutes:     parseInt(document.getElementById('cfg-cooldown').value) || 0,
    webhooks,
  };

  try {
    const res = await fetch('/api/settings', {
      method: 'PUT',
      headers: { 'Content-Type': 'application/json' },
      body: JSON.stringify(payload),
    });
    const data = await res.json();
    if (!res.ok) {
      msg.textContent = data.error || 'Save failed';
      msg.className = 'err';
    } else {
      msg.textContent = 'Settings saved';
      msg.className = 'ok';
      setTimeout(() => { msg.textContent = ''; msg.className = ''; }, 3000);
    }
  } catch (e) {
    msg.textContent = 'Network error: ' + e.message;
    msg.className = 'err';
  } finally {
    btn.disabled = false;
  }
}

document.getElementById('save-btn').addEventListener('click', saveSettings);

// ── Tab switching ─────────────────────────────────────────────────────────

document.querySelectorAll('.tab-btn').forEach(btn => {
  btn.addEventListener('click', () => {
    document.querySelectorAll('.tab-btn').forEach(b => b.classList.remove('active'));
    btn.classList.add('active');
    const tab = btn.dataset.tab;
    document.getElementById('dashboard-tab').style.display = tab === 'dashboard' ? '' : 'none';
    document.getElementById('settings-tab').style.display  = tab === 'settings'  ? '' : 'none';
    if (tab === 'settings') loadSettings();
  });
});
