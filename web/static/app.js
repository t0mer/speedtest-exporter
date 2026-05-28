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
    document.getElementById('dashboard-tab').style.display     = tab === 'dashboard'     ? '' : 'none';
    document.getElementById('settings-tab').style.display      = tab === 'settings'      ? '' : 'none';
    document.getElementById('notifications-tab').style.display = tab === 'notifications' ? '' : 'none';
    if (tab === 'settings')      loadSettings();
    if (tab === 'notifications') loadChannels();
  });
});

// ── Notifications tab ─────────────────────────────────────────────────────

let editingChannelId = null;

async function loadChannels() {
  const channels = await fetch('/api/notifications').then(r => r.json());
  const list = document.getElementById('channels-list');
  if (!channels || channels.length === 0) {
    list.innerHTML = '<p class="no-data">No channels configured yet. Click &quot;+ Add Channel&quot; to add one.</p>';
    return;
  }
  // Build channel rows with data- attributes; wire click handlers to avoid
  // inline onclick string injection.
  list.innerHTML = channels.map(ch => `<div class="channel-row" data-id="${ch.id}" data-name="${esc(ch.name)}">
    <span class="ch-name">${esc(ch.name)}</span>
    <span class="ch-provider">${esc(ch.provider)}</span>
    <span class="ch-badge ${ch.enabled ? 'enabled' : 'disabled'}">${ch.enabled ? 'On' : 'Off'}</span>
    <div class="ch-actions">
      <button class="btn-secondary ch-edit-btn">Edit</button>
      <button class="btn-secondary ch-delete-btn">Delete</button>
    </div>
  </div>`).join('');

  // Wire edit/delete handlers via addEventListener — no inline string interpolation.
  list.querySelectorAll('.ch-edit-btn').forEach(btn => {
    const row = btn.closest('.channel-row');
    btn.addEventListener('click', () => openEditDialog(Number(row.dataset.id)));
  });
  list.querySelectorAll('.ch-delete-btn').forEach(btn => {
    const row = btn.closest('.channel-row');
    btn.addEventListener('click', () => deleteChannel(Number(row.dataset.id), row.dataset.name));
  });
}

function getProviderConfig() {
  const p = document.getElementById('ch-provider').value;
  if (p === 'shoutrrr') {
    return { url: document.getElementById('ch-shoutrrr-url').value.trim() };
  }
  if (p === 'greenapi') {
    return {
      instance_id: document.getElementById('ch-ga-instance').value.trim(),
      token:       document.getElementById('ch-ga-token').value,
      phone:       document.getElementById('ch-ga-phone').value.trim(),
      api_url:     document.getElementById('ch-ga-apiurl').value.trim(),
    };
  }
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
  ['ch-shoutrrr-url','ch-ga-instance','ch-ga-token','ch-ga-phone','ch-ga-apiurl','ch-wa-baseurl','ch-wa-phone','ch-wa-user','ch-wa-pass'].forEach(id => {
    const el = document.getElementById(id); if (el) el.value = '';
  });
  document.getElementById('ch-enabled').checked    = true;
  document.getElementById('ch-on-success').checked = true;
  document.getElementById('ch-on-failure').checked = true;
  document.getElementById('ch-test-msg').textContent = '';
  document.getElementById('channel-dialog').style.display = '';
}

async function openEditDialog(id) {
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
}

async function saveChannel() {
  const provider = document.getElementById('ch-provider').value;
  const payload = {
    name:              document.getElementById('ch-name').value.trim(),
    provider,
    config:            getProviderConfig(),
    enabled:           document.getElementById('ch-enabled').checked,
    notify_on_success: document.getElementById('ch-on-success').checked,
    notify_on_failure: document.getElementById('ch-on-failure').checked,
  };
  if (!payload.name) { alert('Name is required'); return; }

  const url    = editingChannelId ? `/api/notifications/${editingChannelId}` : '/api/notifications';
  const method = editingChannelId ? 'PUT' : 'POST';
  const res = await fetch(url, { method, headers: {'Content-Type':'application/json'}, body: JSON.stringify(payload) });
  const data = await res.json();
  if (!res.ok) { alert('Error: ' + (data.error || res.status)); return; }
  document.getElementById('channel-dialog').style.display = 'none';
  await loadChannels();
}

async function deleteChannel(id, name) {
  if (!confirm(`Delete channel "${name}"?`)) return;
  const res = await fetch(`/api/notifications/${id}`, { method: 'DELETE' });
  if (res.status !== 204 && !res.ok) { alert('Delete failed'); return; }
  await loadChannels();
}

async function sendTestNotification() {
  const provider = document.getElementById('ch-provider').value;
  const msgEl    = document.getElementById('ch-test-msg');
  msgEl.textContent = 'Sending…';

  const payload = editingChannelId
    ? { id: editingChannelId }
    : { provider, config: getProviderConfig() };

  try {
    const res  = await fetch('/api/notifications/test', { method: 'POST', headers: {'Content-Type':'application/json'}, body: JSON.stringify(payload) });
    const data = await res.json();
    msgEl.textContent = res.ok ? '✅ Sent!' : '❌ ' + (data.error || 'Failed');
  } catch(e) {
    msgEl.textContent = '❌ ' + e.message;
  }
}

document.getElementById('add-channel-btn').addEventListener('click', openAddDialog);
document.getElementById('ch-save-btn').addEventListener('click', saveChannel);
document.getElementById('ch-cancel-btn').addEventListener('click', () => {
  document.getElementById('channel-dialog').style.display = 'none';
});
document.getElementById('ch-test-btn').addEventListener('click', sendTestNotification);
document.getElementById('ch-provider').addEventListener('change', e => switchProviderFields(e.target.value));
