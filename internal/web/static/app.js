// Vahti web app. Plain JavaScript, no external libraries.
// Security: the DOM is built with createElement/textContent only (no
// HTML string parsing), so names coming from the network cannot inject markup.
'use strict';
(() => {

// ------------------------------------------------------------------ texts
const CATS = {
  ads:       {n: 'Advertising',       d: 'Ad networks and targeted advertising.'},
  analytics: {n: 'Analytics',    d: 'Usage statistics, clicks and shopping journeys.'},
  telemetry: {n: 'Telemetry',     d: 'Device usage and diagnostic data sent to the manufacturer.', c: 'Remote support from the manufacturer may be limited.'},
  location:  {n: 'Location',       d: 'Services that collect location data.'},
  social:    {n: 'Social tracking',  d: 'Social media pixels and embeds on other companies\' websites and apps.', c: 'Signing in with social media accounts may stop working.'},
  bypass:    {n: 'Bypass attempts', d: 'Encrypted DNS services a device could use to get around Vahti.', c: 'Browsers\' own encrypted DNS is turned off, so queries go through Vahti.'},
  threat:    {n: 'Malicious',    d: 'Malware and phishing servers.', locked: true},
};
const CAT_ORDER = ['ads', 'analytics', 'telemetry', 'location', 'social', 'bypass', 'threat'];
const KINDS = {unknown: 'Unknown', phone: 'Phone', computer: 'Computer', tablet: 'Tablet', tv: 'TV',
  speaker: 'Speaker', camera: 'Camera', appliance: 'Appliance', console: 'Game console', other: 'Other'};
const MODES = {
  open:     {n: 'Open', d: 'Unknown destinations are allowed and known tracking services are blocked. Suits phones and computers.'},
  learning: {n: 'Learning', d: 'The device can connect anywhere, and Vahti saves the destinations as allowed. When learning ends, the device is locked.'},
  locked:   {n: 'Locked', d: 'Only allowed destinations work. New destinations are blocked and brought to you for a decision. Suits smart devices.'},
};
const ICONS = {
  phone: 'M7 2.5h10a2 2 0 0 1 2 2v15a2 2 0 0 1-2 2H7a2 2 0 0 1-2-2v-15a2 2 0 0 1 2-2ZM11 18.5h2',
  computer: 'M4 5h16v10H4zM2 19h20',
  tablet: 'M5 3h14a1 1 0 0 1 1 1v16a1 1 0 0 1-1 1H5a1 1 0 0 1-1-1V4a1 1 0 0 1 1-1ZM11 18h2',
  tv: 'M3 5h18v12H3zM8 20h8',
  speaker: 'M8 2.5h8a2 2 0 0 1 2 2v15a2 2 0 0 1-2 2H8a2 2 0 0 1-2-2v-15a2 2 0 0 1 2-2ZM12 10.5a3.5 3.5 0 1 1 0 7 3.5 3.5 0 0 1 0-7Z',
  camera: 'M9 2.5h6a3 3 0 0 1 3 3v13a3 3 0 0 1-3 3H9a3 3 0 0 1-3-3v-13a3 3 0 0 1 3-3ZM12 6.5a2.5 2.5 0 1 1 0 5 2.5 2.5 0 0 1 0-5Z',
  appliance: 'M5 3h14v18H5zM5 8h14M12 11a4 4 0 1 1 0 8 4 4 0 0 1 0-8Z',
  console: 'M6 8h12a4 4 0 0 1 0 8H6a4 4 0 0 1 0-8ZM8 11v3M6.5 12.5h3M15.5 12h.01M17.5 13.5h.01',
  other: 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18ZM3 12h18',
  unknown: 'M12 3a9 9 0 1 1 0 18 9 9 0 0 1 0-18ZM9.5 9.5a2.5 2.5 0 1 1 3.5 2.3c-.6.3-1 .9-1 1.6v.6M12 17h.01',
};

// ------------------------------------------------------------------ DOM helpers
function h(tag, props, ...kids) {
  const el = document.createElement(tag);
  for (const [k, v] of Object.entries(props || {})) {
    if (v == null || v === false) continue;
    if (k === 'class') el.className = v;
    else if (k === 'style') Object.assign(el.style, v);
    else if (k.startsWith('on')) el.addEventListener(k.slice(2), v);
    else if (k === 'value') el.value = v;
    else if (v === true) el.setAttribute(k, '');
    else el.setAttribute(k, String(v));
  }
  add(el, kids);
  return el;
}
// add appends children to an element: nodes as they are, arrays recursively, and anything
// else as plain text (never as HTML). null and false are skipped, which makes optional parts easy.
function add(el, kids) {
  for (const k of kids) {
    if (k == null || k === false) continue;
    if (Array.isArray(k)) add(el, k);
    else if (k instanceof Node) el.appendChild(k);
    else el.appendChild(document.createTextNode(String(k)));
  }
}
// svgIcon draws one of the line icons (phone, TV, laptop and so on) as an SVG element.
function svgIcon(kind, size = 18) {
  const ns = 'http://www.w3.org/2000/svg';
  const s = document.createElementNS(ns, 'svg');
  for (const [k, v] of Object.entries({width: size, height: size, viewBox: '0 0 24 24', fill: 'none', stroke: 'currentColor',
    'stroke-width': '1.7', 'stroke-linecap': 'round', 'stroke-linejoin': 'round', 'aria-hidden': 'true'})) s.setAttribute(k, v);
  const p = document.createElementNS(ns, 'path');
  p.setAttribute('d', ICONS[kind] || ICONS.unknown);
  s.appendChild(p);
  return s;
}
// The Vahti logo: a detailed version for large sizes, a simplified one below 48 px.
function logo(size) {
  return h('img', {src: size >= 48 ? '/static/logo.svg' : '/static/icon.svg', width: size, height: size, alt: '', class: 'logo'});
}
const $ = (s) => document.querySelector(s);
// Formatting helpers: time of day, date and time, a number with thousands separators,
// and the readable name of a tracker category.
const fmtTime = (t) => new Date(t).toLocaleTimeString('en-GB', {hour: '2-digit', minute: '2-digit', second: '2-digit'});
const fmtDateTime = (t) => new Date(t).toLocaleString('en-GB', {day: 'numeric', month: 'numeric', hour: '2-digit', minute: '2-digit'});
const fmtNum = (n) => Number(n || 0).toLocaleString('en-GB');
const catName = (c) => c ? (CATS[c] ? CATS[c].n : c) : 'Unknown';
// ago turns a timestamp into "just now", "5 min ago", "3 h ago" or "2 d ago".
function ago(t) {
  const s = (Date.now() - new Date(t).getTime()) / 1000;
  if (s < 60) return 'just now';
  if (s < 3600) return Math.floor(s / 60) + ' min ago';
  if (s < 86400) return Math.floor(s / 3600) + ' h ago';
  return Math.floor(s / 86400) + ' d ago';
}

// toast shows a short message at the bottom of the screen for a few seconds.
function toast(msg) {
  const t = $('#toast');
  t.textContent = msg; t.hidden = false;
  clearTimeout(toast.h); toast.h = setTimeout(() => { t.hidden = true; }, 2600);
}

// ------------------------------------------------------------------ API
const S = {session: null, view: 'overview', data: {}, sheet: null, sheetData: null, logFilter: {action: 'all', device: '', q: ''}, busy: false};

// api calls Vahti's JSON API. It sends the CSRF token with every change, and throws an error
// with the server's message if the request fails.
async function api(method, path, body) {
  const opt = {method, credentials: 'same-origin', headers: {}};
  if (body !== undefined) {
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body);
  }
  if (method !== 'GET' && S.session && S.session.csrf) opt.headers['X-CSRF-Token'] = S.session.csrf;
  let r;
  try { r = await fetch(path, opt); } catch (e) { throw new Error('Cannot reach the Vahti device.'); }
  let j = null;
  try { j = await r.json(); } catch (e) { /* empty */ }
  if (r.status === 401 && S.session && S.session.authenticated) {
    S.session.authenticated = false;
    render();
  }
  if (!r.ok) throw new Error((j && j.error) || 'Something went wrong (' + r.status + ').');
  return j;
}

// act runs a change (fn), shows okMsg if it succeeded or the error message if not, and
// reloads the data so the screen is up to date.
async function act(fn, okMsg) {
  try {
    await fn();
    if (okMsg) toast(okMsg);
    await load();
  } catch (e) { toast(e.message); }
}

// ------------------------------------------------------------------ loading
const VIEW_LOAD = {
  overview: async () => ({o: await api('GET', '/api/overview')}),
  devices:  async () => ({devices: await api('GET', '/api/devices')}),
  queue:    async () => ({queue: await api('GET', '/api/queue')}),
  rules:    async () => ({rules: await api('GET', '/api/rules'), settings: await api('GET', '/api/settings'), devices: await api('GET', '/api/devices')}),
  log:      async () => {
    const f = S.logFilter, p = new URLSearchParams({limit: '300'});
    if (f.action !== 'all') p.set('action', f.action);
    if (f.device) p.set('device', f.device);
    if (f.q) p.set('q', f.q);
    return {log: await api('GET', '/api/log?' + p), devices: S.data.devices || await api('GET', '/api/devices')};
  },
  settings: async () => ({settings: await api('GET', '/api/settings')}),
};

// load fetches the data for the current view (plus the overview numbers for the header)
// and redraws the page. If the connection fails, the page shows that Vahti is offline.
async function load() {
  if (!S.session || !S.session.authenticated) return;
  try {
    const d = await VIEW_LOAD[S.view]();
    Object.assign(S.data, d);
    const ov = S.view === 'overview' ? d.o : await api('GET', '/api/overview');
    S.data.o = ov; S.online = true;
    if (S.sheet) S.sheetData = await api('GET', '/api/devices/' + encodeURIComponent(S.sheet));
  } catch (e) {
    S.online = false;
    if (S.sheet && /Not found/.test(e.message)) { S.sheet = null; S.sheetData = null; }
  }
  render();
}

// Auto-refresh, paused while the user is typing or choosing in a form.
setInterval(() => {
  if (document.hidden || !S.session || !S.session.authenticated) return;
  const a = document.activeElement;
  if (a && (a.tagName === 'INPUT' || a.tagName === 'SELECT')) return;
  if (S.view === 'rules' || S.view === 'settings') return;
  if (S.view === 'queue') {
    // Keep the list still while the user reads it; reload only when it changes.
    api('GET', '/api/overview').then((o) => {
      const before = S.data.o && S.data.o.queue;
      S.data.o = o;
      if (o.queue !== before) load(); else render();
    }).catch(() => {});
    return;
  }
  load();
}, 3000);

// ------------------------------------------------------------------ render
function render() {
  const app = $('#app');
  const scroll = window.scrollY;
  const sheetScroll = document.querySelector('.sheet') ? document.querySelector('.sheet').scrollTop : 0;
  app.replaceChildren();
  if (!S.session) { app.append(h('div', {class: 'auth'}, h('p', {class: 'muted'}, 'Loading…'))); return; }
  if (!S.session.authenticated) { app.append(S.session.setupRequired ? vSetup() : vLogin()); return; }
  const o = S.data.o || {};
  const paused = o.pausedUntil && new Date(o.pausedUntil) > new Date();
  app.append(
    S.session.demo ? h('div', {class: 'demo-band'}, 'Demo mode: traffic is simulated, but every rule and decision runs through the real engine.') : '',
    S.session.local ? h('div', {class: 'demo-band'}, 'Experimental local mode: Vahti filters only this computer\'s traffic. DNS settings are restored automatically when Vahti is closed.') : '',
    h('header', {class: 'top'}, h('div', {class: 'wrap'},
      h('div', {class: 'bar'},
        h('div', {class: 'brand'}, logo(28), 'Vahti'),
        h('div', {class: 'status'}, h('span', {class: 'dot' + (S.online === false ? ' down' : paused ? ' paused' : '')}),
          h('span', {}, S.online === false ? 'No connection to Vahti' : paused ? 'Protection paused until ' + fmtTime(o.pausedUntil).slice(0, 5) + '' : 'Protecting your home network')),
        h('div', {class: 'spacer'}),
        h('span', {class: 'local', title: 'This app talks only to the Vahti device on your home network'}, 'Local · ' + location.host),
      ),
      h('nav', {class: 'tabs', role: 'tablist', 'aria-label': 'Views'},
        tab('overview', 'Overview'), tab('devices', 'Devices'),
        tab('queue', 'Waiting for decision', o.queue ? h('span', {class: 'badge'}, o.queue) : null),
        tab('rules', 'Rules'), tab('log', 'Log'), tab('settings', 'Settings')),
    )),
    h('main', {class: 'wrap', id: 'main'}, VIEWS[S.view]()),
  );
  if (S.sheet && S.sheetData) app.append(...vSheet());
  window.scrollTo(0, scroll);
  const sh = document.querySelector('.sheet');
  if (sh) sh.scrollTop = sheetScroll;
}

// tab creates one navigation tab button. Clicking it switches the view and loads its data.
function tab(v, label, extra) {
  return h('button', {role: 'tab', 'aria-selected': S.view === v ? 'true' : 'false', onclick: () => {
    S.view = v; S.data = {o: S.data.o}; render(); load();
  }}, label, extra);
}

// ------------------------------------------------------------------ auth views
function vSetup() {
  const err = h('p', {class: 'err', hidden: true});
  const code = h('input', {id: 'setup-code', autocomplete: 'off', placeholder: 'XXXX-XXXX-XXXX-XXXX', required: true, maxlength: 40});
  const pw = h('input', {id: 'setup-pw', type: 'password', autocomplete: 'new-password', required: true, minlength: 12, maxlength: 256});
  const pw2 = h('input', {id: 'setup-pw2', type: 'password', autocomplete: 'new-password', required: true, maxlength: 256});
  const btn = h('button', {class: 'b primary', type: 'submit'}, 'Set up Vahti');
  const form = h('form', {class: 'grid', onsubmit: async (e) => {
    e.preventDefault(); err.hidden = true;
    if (pw.value !== pw2.value) { err.textContent = 'The passwords do not match.'; err.hidden = false; return; }
    btn.disabled = true;
    try {
      const r = await api('POST', '/api/setup', {code: code.value, password: pw.value});
      S.session = Object.assign(S.session, r); await load();
    } catch (x) { err.textContent = x.message; err.hidden = false; btn.disabled = false; }
  }},
  h('label', {class: 'field', for: 'setup-code'}, 'Setup code', code),
  h('label', {class: 'field', for: 'setup-pw'}, 'New password (at least 12 characters)', pw),
  h('label', {class: 'field', for: 'setup-pw2'}, 'Repeat password', pw2),
  err, btn);
  return h('div', {class: 'auth'}, h('section', {class: 'panel'},
    h('div', {class: 'brand brand-lg'}, logo(64), 'Vahti'),
    h('h1', {}, 'Set up Vahti'),
    h('p', {class: 'muted'}, 'The setup code was printed to the Vahti log during installation. You can see it with: sudo cat /var/lib/vahti/setup-code'),
    form,
    S.session.certFingerprint ? h('div', {class: 'note'},
      h('p', {}, 'Check that the certificate fingerprint shown by your browser matches the one in the Vahti log:'),
      h('p', {class: 'fp'}, S.session.certFingerprint)) : null,
  ));
}

// vLogin builds the sign-in page.
function vLogin() {
  const err = h('p', {class: 'err', hidden: true});
  const pw = h('input', {id: 'login-pw', type: 'password', autocomplete: 'current-password', required: true, maxlength: 256});
  const btn = h('button', {class: 'b primary', type: 'submit'}, 'Sign in');
  setTimeout(() => pw.focus(), 0);
  return h('div', {class: 'auth'}, h('section', {class: 'panel'},
    h('div', {class: 'brand brand-lg'}, logo(64), 'Vahti'),
    h('h1', {}, 'Sign in'),
    S.session.demo ? h('p', {class: 'note'}, 'Demo mode is running.') : null,
    h('form', {class: 'grid', onsubmit: async (e) => {
      e.preventDefault(); err.hidden = true; btn.disabled = true;
      try {
        const r = await api('POST', '/api/login', {password: pw.value});
        S.session = Object.assign(S.session, r); await load();
      } catch (x) { err.textContent = x.message; err.hidden = false; btn.disabled = false; pw.select(); }
    }}, h('label', {class: 'field', for: 'login-pw'}, 'Password', pw), err, btn),
  ));
}

// ------------------------------------------------------------------ overview
function evRow(e) {
  return h('div', {class: 'ev'},
    h('span', {class: 'mono muted'}, fmtTime(e.time)),
    h('div', {class: 'who'},
      h('div', {}, h('b', {}, e.deviceName), ' ', h('span', {class: 'muted'}, e.category ? catName(e.category) : (e.owner || ''))),
      h('div', {class: 'mono muted'}, e.domain)),
    verdictPill(e));
}
// verdictPill shows a log entry's result as a coloured label: Waiting, Allowed or Blocked.
function verdictPill(e) {
  if (e.queued) return h('span', {class: 'pill hold'}, 'Waiting');
  return h('span', {class: 'pill ' + e.action}, e.action === 'allow' ? 'Allowed' : 'Blocked');
}

// vOverview builds the home page: the numbers, a pause notice, how many decisions are
// waiting, live traffic and what was blocked by category.
function vOverview() {
  const o = S.data.o;
  if (!o) return [h('p', {class: 'muted'}, 'Loading…')];
  const cats = Object.entries(o.categories || {}).sort((a, b) => b[1] - a[1]);
  const max = Math.max(1, ...cats.map((c) => c[1]));
  const paused = o.pausedUntil && new Date(o.pausedUntil) > new Date();
  return [
    paused ? h('div', {class: 'alert warn'}, h('p', {}, h('b', {}, 'Protection is paused. '), 'All traffic except malicious destinations is let through until ' + fmtTime(o.pausedUntil).slice(0, 5) + '.'),
      h('button', {class: 'b', onclick: () => act(() => api('POST', '/api/pause', {minutes: 0}), 'Protection resumed')}, 'Resume protection')) : null,
    (o.warnings || []).map((w) => h('div', {class: 'alert info'}, h('p', {}, w))),
    h('div', {class: 'stats'},
      stat('Allowed', fmtNum(o.allowed), 'c-ok', 'queries'),
      stat('Blocked', fmtNum(o.blocked), 'c-bad', 'queries'),
      stat('Waiting', fmtNum(o.queue), 'c-warn', 'decisions'),
      stat('Devices', fmtNum(o.devices), '', 'on your network')),
    o.queue ? h('div', {class: 'qi'}, h('p', {}, h('b', {}, o.queue + ' new destinations are waiting for your decision. '), 'They stay blocked until you decide.'),
      h('div', {class: 'btns'}, h('button', {class: 'b primary', onclick: () => { S.view = 'queue'; render(); load(); }}, 'Review'))) : null,
    h('div', {class: 'grid2'},
      h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Live traffic'), h('span', {class: 'muted small'}, 'updates every 3 s')),
        o.recent && o.recent.length ? h('div', {class: 'feed'}, o.recent.map(evRow))
          : h('p', {class: 'muted'}, 'No queries yet. Traffic appears here once devices use Vahti as their DNS server.')),
      h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'What was blocked'), h('span', {class: 'muted small'}, 'by category')),
        cats.length ? h('div', {class: 'catbars'}, cats.map(([k, v]) => h('div', {class: 'catrow'},
          h('span', {}, catName(k)), h('div', {class: 'track'}, h('div', {class: 'fill', style: {width: (v / max * 100) + '%'}})), h('span', {class: 'num'}, fmtNum(v)))))
          : h('p', {class: 'muted'}, 'Nothing blocked yet.'),
        h('p', {class: 'note'}, 'Vahti sees which service a device tries to reach, but not the content of encrypted traffic. Blocking happens before the connection opens, so a blocked request never leaves your home network.')),
    ),
  ];
}
// stat builds one number tile on the home page (label, value and unit).
function stat(label, value, cls, unit) {
  return h('div', {class: 'stat'}, h('span', {class: 'eyebrow'}, label), h('b', {class: cls}, value), h('span', {class: 'muted'}, unit));
}

// ------------------------------------------------------------------ devices
function vDevices() {
  const ds = S.data.devices;
  if (!ds) return [h('p', {class: 'muted'}, 'Loading…')];
  if (!ds.length) return [h('div', {class: 'empty'}, h('b', {}, 'No devices yet. '), 'Devices appear here when they make their first DNS query through Vahti.')];
  return [
    h('div', {class: 'panel-h'}, h('h2', {}, 'Devices on your network'), h('span', {class: 'muted small'}, ds.length + ' devices · select a device to adjust its permissions')),
    h('div', {class: 'devices'}, ds.map((d) => {
      const tot = (d.allowed + d.blocked) || 1;
      return h('button', {class: 'dev', type: 'button', onclick: () => openSheet(d.id)},
        h('div', {class: 'dev-h'}, h('span', {class: 'ico'}, svgIcon(d.kind)),
          h('div', {}, h('b', {}, d.name), h('div', {class: 'muted small'}, KINDS[d.kind] + ' · ' + MODES[d.mode].n))),
        h('div', {class: 'ratio', 'aria-label': 'Allowed ' + d.allowed + ', blocked ' + d.blocked},
          h('i', {class: 'a', style: {width: (d.allowed / tot * 100) + '%'}}), h('i', {class: 'b', style: {width: (d.blocked / tot * 100) + '%'}})),
        h('div', {class: 'dev-meta'}, h('span', {}, h('b', {}, fmtNum(d.allowed)), ' allowed'), h('span', {}, h('b', {}, fmtNum(d.blocked)), ' blocked'),
          h('span', {}, ago(d.lastSeen))));
    })),
  ];
}

// openSheet loads one device's details and opens them in the side panel.
async function openSheet(id) {
  S.sheet = id;
  try { S.sheetData = await api('GET', '/api/devices/' + encodeURIComponent(id)); } catch (e) { toast(e.message); S.sheet = null; }
  render();
}
// closeSheet closes the device side panel. The Escape key closes it too.
function closeSheet() { S.sheet = null; S.sheetData = null; render(); }
document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && S.sheet) closeSheet(); });

// vSheet builds the device side panel: name and type, how the device may use the network
// (open, learning, locked), what it may share (per-category settings), the destinations it
// has contacted with a rule for each, and a button to forget the device.
function vSheet() {
  const {device: d, destinations, rules, defaults} = S.sheetData;
  const nameIn = h('input', {id: 'dev-name', value: d.name, maxlength: 60});
  const kindSel = h('select', {id: 'dev-kind', onchange: () => patch({kind: kindSel.value}, 'Type saved')},
    Object.entries(KINDS).map(([k, v]) => h('option', {value: k, selected: d.kind === k}, v)));
  // patch saves a change to this device; ruleFor finds the device's rule for a domain.
  const patch = (body, msg) => act(() => api('PATCH', '/api/devices/' + encodeURIComponent(d.id), body), msg);
  const learnLeft = d.mode === 'learning' && d.learnUntil ? ' Learning ends ' + fmtDateTime(d.learnUntil) + '.' : '';
  const ruleFor = (dom) => (rules || []).find((r) => r.domain === dom);
  return [
    h('div', {class: 'sheet-bg', onclick: closeSheet}),
    h('aside', {class: 'sheet', role: 'dialog', 'aria-modal': 'true', 'aria-label': d.name},
      h('div', {class: 'sheet-h'}, h('span', {class: 'ico'}, svgIcon(d.kind)),
        h('div', {}, h('h2', {}, d.name), h('div', {class: 'mono muted'}, d.ip + (d.mac ? ' · ' + d.mac : ''))),
        h('button', {class: 'x', onclick: closeSheet, 'aria-label': 'Close'}, '✕')),
      h('section', {class: 'panel'},
        h('form', {class: 'row', onsubmit: (e) => { e.preventDefault(); patch({name: nameIn.value}, 'Name saved'); }},
          h('label', {class: 'field', for: 'dev-name'}, 'Name', nameIn), h('button', {class: 'b', type: 'submit'}, 'Save')),
        h('label', {class: 'field', for: 'dev-kind'}, 'Device type', kindSel)),
      h('section', {class: 'panel'}, h('h2', {}, 'How this device may use the network'),
        h('div', {class: 'modes'}, Object.entries(MODES).map(([k, m]) => h('button', {class: 'mode', type: 'button', 'aria-pressed': d.mode === k ? 'true' : 'false',
          onclick: () => patch(k === 'learning' ? {mode: k, learnHours: 48} : {mode: k}, 'Mode: ' + m.n)},
          h('b', {}, m.n), h('p', {}, m.d + (d.mode === k ? learnLeft : '')))))),
      h('section', {class: 'panel'}, h('h2', {}, 'What this device may share'),
        h('div', {class: 'rules'}, CAT_ORDER.filter((c) => c !== 'threat').map((c) => {
          const ov = (d.catOverride || {})[c] || '';
          return h('div', {class: 'rule'}, h('div', {}, h('h3', {}, CATS[c].n), h('p', {}, 'Default: ' + (defaults[c] === 'allow' ? 'allowed' : 'blocked'))),
            seg(ov, [['', 'Default', 'd'], ['allow', 'Allow', 'a'], ['block', 'Block', 'x']], (v) => {
              const body = {catOverride: {}}; body.catOverride[c] = v || null;
              patch(body, CATS[c].n + ': ' + (v === 'allow' ? 'allowed' : v === 'block' ? 'blocked' : 'default'));
            }));
        }))),
      h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Destinations this device has contacted'), h('span', {class: 'muted small'}, 'from recent queries')),
        destinations.length ? h('div', {class: 'rules'}, destinations.map((x) => {
          const r = ruleFor(x.domain);
          return h('div', {class: 'rule'},
            h('div', {}, h('div', {class: 'mono'}, x.domain),
              h('p', {}, [catName(x.category), x.owner, fmtNum(x.allowed) + ' allowed', fmtNum(x.blocked) + ' blocked'].filter(Boolean).join(' · '),
                r ? ' · ' + (r.action === 'allow' ? 'your permission' : 'your block') + (r.until ? ' until ' + fmtTime(r.until).slice(0, 5) + '' : '') : '')),
            x.category === 'threat' ? null : h('div', {class: 'btns'},
              x.blocked ? h('button', {class: 'b small', onclick: () => act(() => api('POST', '/api/rules', {device: d.id, domain: x.domain, action: 'allow', hours: 1}), 'Shared for 1 hour')}, 'Share for 1 hour') : null,
              x.allowed ? h('button', {class: 'b small danger', onclick: () => act(() => api('POST', '/api/rules', {device: d.id, domain: x.domain, action: 'block', hours: 0}), 'Blocked for this device')}, 'Block') : null));
        })) : h('p', {class: 'muted'}, 'No traffic in recent queries.')),
      (rules || []).length ? h('section', {class: 'panel'}, h('h2', {}, 'This device\'s rules'), rulesList(rules)) : null,
      h('section', {class: 'panel'}, h('h2', {}, 'Forget device'), h('p', {class: 'muted small'}, 'Removes the device, its rules and pending decisions. The device comes back as new if it uses the network again.'),
        armedButton('Forget device', 'Confirm: forget device', async () => {
          try { await api('DELETE', '/api/devices/' + encodeURIComponent(d.id)); toast('Device forgotten'); closeSheet(); load(); } catch (e) { toast(e.message); }
        })),
    ),
  ];
}

// seg builds a segmented control (a row of buttons where one is selected), e.g. Allow/Block.
function seg(val, opts, onpick, disabled) {
  return h('div', {class: 'seg', role: 'group'}, opts.map(([v, l, c]) => h('button', {type: 'button', class: c, 'aria-pressed': val === v ? 'true' : 'false',
    disabled: !!disabled, onclick: () => { if (val !== v) onpick(v); }}, l)));
}

// A destructive button that asks for a second click instead of a dialog.
function armedButton(label, confirmLabel, fn) {
  const b = h('button', {class: 'b danger', type: 'button'}, label);
  let armed = false, t;
  b.addEventListener('click', () => {
    if (!armed) { armed = true; b.textContent = confirmLabel; b.classList.add('armed'); t = setTimeout(() => { armed = false; b.textContent = label; b.classList.remove('armed'); }, 4000); return; }
    clearTimeout(t); fn();
  });
  return b;
}

// ------------------------------------------------------------------ queue
function vQueue() {
  const q = S.data.queue;
  if (!q) return [h('p', {class: 'muted'}, 'Loading…')];
  if (!q.length) return [h('div', {class: 'empty'}, h('b', {}, 'No pending decisions. '),
    'When a locked device tries to reach a new destination, it appears here and stays blocked until you decide. Lock smart devices in the Devices view.')];
  return [
    h('div', {class: 'panel-h'}, h('h2', {}, 'Waiting for decision'), h('span', {class: 'muted small'}, 'Everything below stays blocked until you allow it.')),
    h('div', {class: 'q'}, q.map(qCard)),
  ];
}

// qCard builds one card in the decision queue: what the device tried to reach, who owns it,
// what it is, related addresses and devices, and the decision buttons (allow once, for a while,
// always, or block). The checkbox applies the decision to the whole site.
function qCard(it) {
  const siteId = 'site-' + it.id;
  const site = h('input', {type: 'checkbox', id: siteId});
  // decide sends the user's choice for this queue item, for this name only or the whole site.
  const decide = (decision, msg) => act(() => api('POST', '/api/queue/' + encodeURIComponent(it.id), {decision, scope: site.checked ? 'site' : 'name'}), msg);
  const facts = [
    ['Owner', it.owner ? h('b', {}, it.owner) : h('span', {class: 'muted'}, 'Not in the local database')],
    ['Site', h('span', {class: 'mono'}, it.registrable)],
    it.category ? ['Category', [catName(it.category), it.listSource ? h('span', {class: 'muted'}, ' · source: ' + it.listSource) : null]] : null,
    it.note ? ['What it is', it.note] : null,
    it.siblings && it.siblings.length ? ['Other addresses on the same site', h('ul', {}, it.siblings.map((s) => h('li', {class: 'mono'}, s)))] : null,
    ['Other devices', it.otherDevices && it.otherDevices.length ? it.otherDevices.join(', ') + ' also contacted this destination' : 'Only this device'],
    it.vias && it.vias.length ? ['Redirects to', it.vias.join(', ')] : null,
    ['Attempts', it.count + ' times · first ' + fmtDateTime(it.first) + ' · last ' + ago(it.last)],
  ].filter(Boolean);
  return h('div', {class: 'qi'},
    h('div', {class: 'dev-h'}, h('span', {class: 'ico'}, svgIcon(it.deviceKind)),
      h('div', {}, h('b', {}, it.deviceName), ' is trying to reach', h('div', {class: 'mono'}, it.domain))),
    h('div', {class: 'intel'}, h('span', {class: 'eyebrow'}, 'What this destination is linked to'),
      h('dl', {}, facts.map(([k, v]) => [h('dt', {}, k), h('dd', {}, v)])),
      h('p', {class: 'muted small'}, 'This information comes from the Vahti device\'s own database and your home network traffic. Nothing about the destination is looked up on the internet.')),
    h('label', {class: 'check', for: siteId}, site, 'Apply the decision to the whole site ', h('span', {class: 'mono'}, it.registrable)),
    h('div', {class: 'btns'},
      h('button', {class: 'b', onclick: () => decide('once', 'Allowed once')}, 'Allow once'),
      h('button', {class: 'b', onclick: () => decide('hour', 'Allowed for 1 hour')}, 'Allow for 1 hour'),
      h('button', {class: 'b', onclick: () => decide('allow', 'Always allowed')}, 'Always allow'),
      h('button', {class: 'b primary', onclick: () => decide('block', 'Always blocked')}, 'Always block')));
}

// ------------------------------------------------------------------ rules
function rulesList(rules) {
  return h('div', {class: 'rules'}, rules.map((r) => h('div', {class: 'rule'},
    h('div', {}, h('div', {}, h('span', {class: 'mono'}, r.domain), ' ', h('span', {class: 'pill ' + r.action}, r.action === 'allow' ? 'Allowed' : 'Blocked')),
      h('p', {}, [r.deviceName,
        r.once ? 'next time only' : r.until ? 'until ' + fmtDateTime(r.until) + '' : 'permanent',
        {user: 'your rule', queue: 'decision from queue', learned: 'learned', share: 'shared'}[r.source] || r.source].join(' · '))),
    h('button', {class: 'b small danger', onclick: () => act(() => api('DELETE', '/api/rules/' + encodeURIComponent(r.id)), 'Rule removed')}, 'Remove'))));
}

// vRules builds the Rules page: category defaults for all devices, the mode for new devices,
// a form for adding a rule by hand, and the lists of your own rules and learned destinations.
function vRules() {
  const {settings: st, rules, devices} = S.data;
  if (!st || !rules) return [h('p', {class: 'muted'}, 'Loading…')];
  const domIn = h('input', {id: 'rule-domain', placeholder: 'e.g. tracker.example.com', maxlength: 253, required: true});
  const devSel = h('select', {id: 'rule-device'}, h('option', {value: '*'}, 'All devices'), (devices || []).map((d) => h('option', {value: d.id}, d.name)));
  const actSel = h('select', {id: 'rule-action'}, h('option', {value: 'block'}, 'Block'), h('option', {value: 'allow'}, 'Allow'));
  const userRules = rules.filter((r) => r.source !== 'learned');
  const learned = rules.filter((r) => r.source === 'learned');
  return [h('div', {class: 'grid2'},
    h('section', {class: 'panel'}, h('h2', {}, 'Defaults for all devices'),
      h('div', {class: 'rules'}, CAT_ORDER.map((c) => h('div', {class: 'rule'},
        h('div', {}, h('h3', {}, CATS[c].n), h('p', {}, CATS[c].d),
          CATS[c].c && st.categories[c] === 'block' ? h('p', {class: 'warnnote'}, CATS[c].c) : null,
          CATS[c].locked ? h('p', {}, 'Always blocked.') : null),
        seg(st.categories[c], [['allow', 'Allow', 'a'], ['block', 'Block', 'x']], (v) => {
          const cats = {}; cats[c] = v;
          act(() => api('PUT', '/api/settings', {categories: cats}), CATS[c].n + ': ' + (v === 'allow' ? 'allowed' : 'blocked'));
        }, CATS[c].locked))))),
    h('div', {class: 'stack'},
      h('section', {class: 'panel'}, h('h2', {}, 'New devices'),
        h('p', {class: 'muted small'}, 'The mode a new device starts in when it appears on the network.'),
        seg(st.newDeviceMode, Object.entries(MODES).map(([k, m]) => [k, m.n, 'd']), (v) => act(() => api('PUT', '/api/settings', {newDeviceMode: v}), 'New devices: ' + MODES[v].n))),
      h('section', {class: 'panel'}, h('h2', {}, 'Add a rule'),
        h('form', {class: 'grid', onsubmit: (e) => {
          e.preventDefault();
          act(() => api('POST', '/api/rules', {device: devSel.value, domain: domIn.value.trim(), action: actSel.value, hours: 0}), 'Rule added').then(() => { domIn.value = ''; });
        }},
        h('label', {class: 'field', for: 'rule-domain'}, 'Domain (also covers subdomains)', domIn),
        h('div', {class: 'row'}, h('label', {class: 'field', for: 'rule-device'}, 'Device', devSel), h('label', {class: 'field', for: 'rule-action'}, 'Action', actSel),
          h('button', {class: 'b primary', type: 'submit'}, 'Add')))),
      h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Your rules and permissions'), h('span', {class: 'muted small'}, String(userRules.length))),
        userRules.length ? rulesList(userRules) : h('p', {class: 'muted'}, 'Rules are created when you decide on a waiting destination, share data from a device page or add a rule yourself.')),
      learned.length ? h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Learned destinations'), h('span', {class: 'muted small'}, String(learned.length))), rulesList(learned)) : null,
    ))];
}

// ------------------------------------------------------------------ log
function vLog() {
  const f = S.logFilter;
  const rows = S.data.log;
  const search = h('input', {id: 'log-q', type: 'search', placeholder: 'Search domains', value: f.q, maxlength: 100,
    oninput: () => { f.q = search.value.trim(); clearTimeout(vLog.t); vLog.t = setTimeout(load, 300); }});
  const devSel = h('select', {id: 'log-device', 'aria-label': 'Device', onchange: () => { f.device = devSel.value; load(); }},
    h('option', {value: ''}, 'All devices'), (S.data.devices || []).map((d) => h('option', {value: d.id, selected: f.device === d.id}, d.name)));
  return [h('section', {class: 'panel'},
    h('div', {class: 'panel-h'}, h('h2', {}, 'Log'),
      h('div', {class: 'filters'},
        h('div', {class: 'chips'}, [['all', 'All'], ['allow', 'Allowed'], ['block', 'Blocked']].map(([v, l]) =>
          h('button', {type: 'button', 'aria-pressed': f.action === v ? 'true' : 'false', onclick: () => { f.action = v; load(); }}, l))),
        devSel, search)),
    !rows ? h('p', {class: 'muted'}, 'Loading…') : !rows.length ? h('p', {class: 'muted'}, 'No matches.') :
      h('div', {class: 'tablewrap'}, h('table', {},
        h('thead', {}, h('tr', {}, ['Time', 'Device', 'Destination', 'Type', 'Category', 'Decision', 'Reason'].map((t) => h('th', {}, t)))),
        h('tbody', {}, rows.map((e) => h('tr', {},
          h('td', {class: 'mono'}, fmtTime(e.time)), h('td', {}, e.deviceName),
          h('td', {class: 'mono'}, e.domain, e.via ? h('div', {class: 'muted'}, '→ ' + e.via) : null),
          h('td', {class: 'mono muted'}, e.qtype),
          h('td', {}, h('span', {class: 'chip'}, e.category ? catName(e.category) : (e.owner || 'Unknown'))),
          h('td', {}, verdictPill(e)), h('td', {class: 'muted'}, e.reason)))))),
    h('p', {class: 'note'}, 'The log is kept only in the Vahti device\'s memory. It is never written to disk or sent anywhere, and a restart clears it.'),
  )];
}

// ------------------------------------------------------------------ settings
function vSettings() {
  const st = S.data.settings;
  if (!st) return [h('p', {class: 'muted'}, 'Loading…')];
  const paused = st.pausedUntil && new Date(st.pausedUntil) > new Date();
  const cur = h('input', {id: 'pw-cur', type: 'password', autocomplete: 'current-password', required: true, maxlength: 256});
  const nw = h('input', {id: 'pw-new', type: 'password', autocomplete: 'new-password', required: true, minlength: 12, maxlength: 256});
  const nw2 = h('input', {id: 'pw-new2', type: 'password', autocomplete: 'new-password', required: true, maxlength: 256});
  // pause pauses protection for m minutes (0 resumes it).
  const pause = (m, msg) => act(() => api('POST', '/api/pause', {minutes: m}), msg);
  return [h('div', {class: 'grid2'},
    h('div', {class: 'stack'},
      h('section', {class: 'panel'}, h('h2', {}, 'Pause'),
        h('p', {class: 'muted small'}, 'If a service stops working, you can pause filtering for a while. Malicious destinations stay blocked during a pause.'),
        paused ? h('div', {class: 'btns'}, h('span', {}, 'Paused until ' + fmtTime(st.pausedUntil).slice(0, 5) + ''), h('button', {class: 'b primary', onclick: () => pause(0, 'Protection resumed')}, 'Resume protection'))
          : h('div', {class: 'btns'}, h('button', {class: 'b', onclick: () => pause(15, 'Paused for 15 minutes')}, '15 min'), h('button', {class: 'b', onclick: () => pause(60, 'Paused for 1 hour')}, '1 hour'))),
      h('section', {class: 'panel'}, h('h2', {}, 'Change password'),
        h('form', {class: 'grid', onsubmit: (e) => {
          e.preventDefault();
          if (nw.value !== nw2.value) { toast('The new passwords do not match.'); return; }
          act(() => api('POST', '/api/password', {current: cur.value, next: nw.value}), 'Password changed. Other sessions were signed out.').then(() => { cur.value = nw.value = nw2.value = ''; });
        }},
        h('label', {class: 'field', for: 'pw-cur'}, 'Current password', cur),
        h('label', {class: 'field', for: 'pw-new'}, 'New password (at least 12 characters)', nw),
        h('label', {class: 'field', for: 'pw-new2'}, 'Repeat new password', nw2),
        h('div', {class: 'btns'}, h('button', {class: 'b primary', type: 'submit'}, 'Change password')))),
      h('section', {class: 'panel'}, h('h2', {}, 'Sign out'),
        h('div', {class: 'btns'}, h('button', {class: 'b', onclick: async () => {
          try { await api('POST', '/api/logout', {}); } catch (e) { /* ignore */ }
          S.session.authenticated = false; S.session.csrf = null; S.data = {}; render();
        }}, 'Sign out'))),
    ),
    h('div', {class: 'stack'},
      h('section', {class: 'panel'}, h('h2', {}, 'Tracker databases'),
        h('p', {class: 'muted small'}, 'Lists are downloaded only when you ask (the update-lists command). Otherwise Vahti never contacts anything on its own.'),
        h('div', {class: 'rules'}, (st.lists || []).map((l) => h('div', {class: 'rule'},
          h('div', {}, h('h3', {}, l.name === 'Vahti' ? 'Built-in list' : l.name), h('p', {}, l.error ? l.error : fmtNum(l.entries) + ' domains' + (l.category !== 'mixed' ? ' · ' + catName(l.category) : ''))),
          h('span', {class: 'pill ' + (l.error ? 'hold' : 'allow')}, l.error ? 'Not downloaded' : 'In use'))))),
      h('section', {class: 'panel'}, h('h2', {}, 'DNS servers'),
        h('p', {class: 'muted small'}, 'Allowed queries are forwarded to these servers encrypted. Blocked queries never leave your home network.'),
        h('ul', {class: 'mono'}, (st.upstreams || []).map((u) => h('li', {}, u))),
        h('p', {class: 'muted small'}, 'Version ' + (st.version || '?'))),
    ))];
}

const VIEWS = {overview: vOverview, devices: vDevices, queue: vQueue, rules: vRules, log: vLog, settings: vSettings};

// ------------------------------------------------------------------ boot
(async function boot() {
  try { S.session = await api('GET', '/api/session'); } catch (e) { S.session = {authenticated: false, setupRequired: false}; toast(e.message); }
  render();
  load();
})();

})();
