// DNS.1 — Shared Application Utilities

const API = '/api/v1';

// ── Toast Notification ──
function toast(msg, type) {
  let t = document.getElementById('toast');
  if (!t) {
    t = document.createElement('div');
    t.id = 'toast';
    t.className = 'toast';
    document.body.appendChild(t);
  }
  t.textContent = msg;
  t.style.display = 'block';
  if (type === 'error') t.style.background = '#dc2626';
  else t.style.background = '#1e2128';
  setTimeout(() => { t.style.display = 'none'; }, 2500);
}

// ── API Helper ──
async function api(path, opts = {}) {
  const token = localStorage.getItem('dns1_token');
  const headers = { 'Content-Type': 'application/json' };
  if (token) headers['Authorization'] = 'Bearer ' + token;
  const r = await fetch(API + path, { headers, ...opts });
  const data = await r.json();
  if (!r.ok) throw new Error(data.details || data.error || `HTTP ${r.status}`);
  return data;
}

// ── Auth ──
function saveAuth(token) {
  localStorage.setItem('dns1_token', token);
  window.location.href = '/dashboard';
}

function logout() {
  localStorage.removeItem('dns1_token');
  window.location.href = '/login';
}

function getToken() {
  return localStorage.getItem('dns1_token');
}

// ── Profile Helpers ──
let currentProfile = 'test';
let profiles = [];
let allowlist = [];
let denylist = [];
let timeWindows = [];

async function loadProfiles() {
  try {
    const data = await api('/profiles');
    profiles = data.profiles || [];
    if (profiles.length > 0 && !profiles.find(p => p.profile_id === currentProfile)) {
      currentProfile = profiles[0].profile_id;
    }
    const sel = document.getElementById('profileSelect');
    if (sel) {
      sel.innerHTML = profiles.map(p =>
        `<option value="${p.profile_id}" ${p.profile_id === currentProfile ? 'selected' : ''}>${p.name || p.profile_id}</option>`
      ).join('');
    }
  } catch (e) { console.error('loadProfiles:', e); }
}

function onProfileChange() {
  const sel = document.getElementById('profileSelect');
  if (sel) {
    currentProfile = sel.value;
    refreshAll();
  }
}

function refreshAll() {
  loadParentalControl();
  loadAllowlist();
  loadDenylist();
  loadTimeWindows();
  loadHistory();
  loadStats();
}

// ── Theme / UI ──
// Note: Each page (dashboard, login) defines its own showTab function
// with page-specific selectors. Nothing needed here.
