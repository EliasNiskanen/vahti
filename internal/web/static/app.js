// Vahti web app. Plain JavaScript, no external libraries.
// Security: the DOM is built with createElement/textContent only (no
// HTML string parsing), so names coming from the network cannot inject markup.
'use strict';
(() => {

// ------------------------------------------------------------------ texts
const CATS = {
  ads:       {n: 'Mainonta',       d: 'Mainosverkot ja kohdennettu mainonta.'},
  analytics: {n: 'Analytiikka',    d: 'Käyttötilastot, klikkaukset ja ostopolut.'},
  telemetry: {n: 'Telemetria',     d: 'Laitteiden käyttö- ja diagnostiikkadata valmistajalle.', c: 'Valmistajan etätuki voi heikentyä.'},
  location:  {n: 'Sijainti',       d: 'Sijaintitietoja keräävät palvelut.'},
  social:    {n: 'Some-seuranta',  d: 'Somepalvelujen pikselit ja upotukset muiden yritysten sivuilla ja sovelluksissa.', c: 'Kirjautuminen somen tunnuksilla voi lakata toimimasta.'},
  bypass:    {n: 'Ohitusyritykset', d: 'Salatut DNS-palvelut, joilla laite voisi kiertää Vahdin.', c: 'Selainten oma salattu DNS kytkeytyy pois, ja kyselyt kulkevat Vahdin kautta.'},
  threat:    {n: 'Haitallinen',    d: 'Haittaohjelma- ja kalastelupalvelimet.', locked: true},
};
const CAT_ORDER = ['ads', 'analytics', 'telemetry', 'location', 'social', 'bypass', 'threat'];
const KINDS = {unknown: 'Tuntematon', phone: 'Puhelin', computer: 'Tietokone', tablet: 'Tabletti', tv: 'Televisio',
  speaker: 'Kaiutin', camera: 'Kamera', appliance: 'Kodinkone', console: 'Pelikonsoli', other: 'Muu'};
const MODES = {
  open:     {n: 'Avoin', d: 'Tuntemattomat kohteet sallitaan, ja tunnetut seurantapalvelut estetään. Sopii puhelimille ja tietokoneille.'},
  learning: {n: 'Opettelee', d: 'Laite saa ottaa yhteyttä minne tahansa, ja Vahti tallentaa kohteet sallituiksi. Opettelun jälkeen laite lukitaan.'},
  locked:   {n: 'Lukittu', d: 'Vain sallitut kohteet toimivat. Uudet kohteet estetään ja tuodaan päätettäviksi. Sopii älylaitteille.'},
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
function add(el, kids) {
  for (const k of kids) {
    if (k == null || k === false) continue;
    if (Array.isArray(k)) add(el, k);
    else if (k instanceof Node) el.appendChild(k);
    else el.appendChild(document.createTextNode(String(k)));
  }
}
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
function shield() {
  const ns = 'http://www.w3.org/2000/svg';
  const s = document.createElementNS(ns, 'svg');
  s.setAttribute('width', '26'); s.setAttribute('height', '26'); s.setAttribute('viewBox', '0 0 26 26'); s.setAttribute('aria-hidden', 'true');
  const a = document.createElementNS(ns, 'path');
  a.setAttribute('d', 'M13 2 4 5.5v6.6c0 5.6 3.8 10.4 9 11.9 5.2-1.5 9-6.3 9-11.9V5.5L13 2Z'); a.setAttribute('fill', 'var(--accent)');
  const b = document.createElementNS(ns, 'path');
  b.setAttribute('d', 'M8.5 13.2 11.8 16.5 17.8 9.8'); b.setAttribute('stroke', 'var(--accent-fg)'); b.setAttribute('stroke-width', '2.2');
  b.setAttribute('fill', 'none'); b.setAttribute('stroke-linecap', 'round'); b.setAttribute('stroke-linejoin', 'round');
  s.append(a, b);
  return s;
}
const $ = (s) => document.querySelector(s);
const fmtTime = (t) => new Date(t).toLocaleTimeString('fi-FI', {hour: '2-digit', minute: '2-digit', second: '2-digit'});
const fmtDateTime = (t) => new Date(t).toLocaleString('fi-FI', {day: 'numeric', month: 'numeric', hour: '2-digit', minute: '2-digit'});
const fmtNum = (n) => Number(n || 0).toLocaleString('fi-FI');
const catName = (c) => c ? (CATS[c] ? CATS[c].n : c) : 'Tuntematon';
function ago(t) {
  const s = (Date.now() - new Date(t).getTime()) / 1000;
  if (s < 60) return 'juuri nyt';
  if (s < 3600) return Math.floor(s / 60) + ' min sitten';
  if (s < 86400) return Math.floor(s / 3600) + ' h sitten';
  return Math.floor(s / 86400) + ' pv sitten';
}

function toast(msg) {
  const t = $('#toast');
  t.textContent = msg; t.hidden = false;
  clearTimeout(toast.h); toast.h = setTimeout(() => { t.hidden = true; }, 2600);
}

// ------------------------------------------------------------------ API
const S = {session: null, view: 'overview', data: {}, sheet: null, sheetData: null, logFilter: {action: 'all', device: '', q: ''}, busy: false};

async function api(method, path, body) {
  const opt = {method, credentials: 'same-origin', headers: {}};
  if (body !== undefined) {
    opt.headers['Content-Type'] = 'application/json';
    opt.body = JSON.stringify(body);
  }
  if (method !== 'GET' && S.session && S.session.csrf) opt.headers['X-CSRF-Token'] = S.session.csrf;
  let r;
  try { r = await fetch(path, opt); } catch (e) { throw new Error('Vahti-laitteeseen ei saada yhteyttä.'); }
  let j = null;
  try { j = await r.json(); } catch (e) { /* empty */ }
  if (r.status === 401 && S.session && S.session.authenticated) {
    S.session.authenticated = false;
    render();
  }
  if (!r.ok) throw new Error((j && j.error) || 'Jokin meni vikaan (' + r.status + ').');
  return j;
}

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
    if (S.sheet && /löytynyt/.test(e.message)) { S.sheet = null; S.sheetData = null; }
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
  if (!S.session) { app.append(h('div', {class: 'auth'}, h('p', {class: 'muted'}, 'Ladataan…'))); return; }
  if (!S.session.authenticated) { app.append(S.session.setupRequired ? vSetup() : vLogin()); return; }
  const o = S.data.o || {};
  const paused = o.pausedUntil && new Date(o.pausedUntil) > new Date();
  app.append(
    S.session.demo ? h('div', {class: 'demo-band'}, 'Esittelytila: liikenne on simuloitua, mutta kaikki säännöt ja päätökset toimivat oikean moottorin läpi.') : null,
    S.session.local ? h('div', {class: 'demo-band'}, 'Kokeellinen paikallinen tila: Vahti suodattaa vain tämän tietokoneen liikenteen. DNS-asetukset palautuvat automaattisesti, kun Vahti suljetaan.') : null,
    h('header', {class: 'top'}, h('div', {class: 'wrap'},
      h('div', {class: 'bar'},
        h('div', {class: 'brand'}, shield(), 'Vahti'),
        h('div', {class: 'status'}, h('span', {class: 'dot' + (S.online === false ? ' down' : paused ? ' paused' : '')}),
          h('span', {}, S.online === false ? 'Ei yhteyttä Vahtiin' : paused ? 'Suojaus tauolla klo ' + fmtTime(o.pausedUntil).slice(0, 5) + ' asti' : 'Valvoo kotiverkkoa')),
        h('div', {class: 'spacer'}),
        h('span', {class: 'local', title: 'Sovellus puhuu vain kotiverkon Vahti-laitteelle'}, 'Paikallinen · ' + location.host),
      ),
      h('nav', {class: 'tabs', role: 'tablist', 'aria-label': 'Näkymät'},
        tab('overview', 'Yleiskuva'), tab('devices', 'Laitteet'),
        tab('queue', 'Odottaa päätöstä', o.queue ? h('span', {class: 'badge'}, o.queue) : null),
        tab('rules', 'Säännöt'), tab('log', 'Loki'), tab('settings', 'Asetukset')),
    )),
    h('main', {class: 'wrap', id: 'main'}, VIEWS[S.view]()),
  );
  if (S.sheet && S.sheetData) app.append(...vSheet());
  window.scrollTo(0, scroll);
  const sh = document.querySelector('.sheet');
  if (sh) sh.scrollTop = sheetScroll;
}

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
  const btn = h('button', {class: 'b primary', type: 'submit'}, 'Ota Vahti käyttöön');
  const form = h('form', {class: 'grid', onsubmit: async (e) => {
    e.preventDefault(); err.hidden = true;
    if (pw.value !== pw2.value) { err.textContent = 'Salasanat eivät täsmää.'; err.hidden = false; return; }
    btn.disabled = true;
    try {
      const r = await api('POST', '/api/setup', {code: code.value, password: pw.value});
      S.session = Object.assign(S.session, r); await load();
    } catch (x) { err.textContent = x.message; err.hidden = false; btn.disabled = false; }
  }},
  h('label', {class: 'field', for: 'setup-code'}, 'Käyttöönottokoodi', code),
  h('label', {class: 'field', for: 'setup-pw'}, 'Uusi salasana (vähintään 12 merkkiä)', pw),
  h('label', {class: 'field', for: 'setup-pw2'}, 'Salasana uudelleen', pw2),
  err, btn);
  return h('div', {class: 'auth'}, h('section', {class: 'panel'},
    h('div', {class: 'brand'}, shield(), 'Vahti'),
    h('h1', {}, 'Ota Vahti käyttöön'),
    h('p', {class: 'muted'}, 'Käyttöönottokoodi tulostui Vahti-laitteen lokiin asennuksen yhteydessä. Näet sen komennolla: sudo cat /var/lib/vahti/setup-code'),
    form,
    S.session.certFingerprint ? h('div', {class: 'note'},
      h('p', {}, 'Varmista, että selaimen näyttämä varmenteen sormenjälki on sama kuin Vahti-laitteen lokissa:'),
      h('p', {class: 'fp'}, S.session.certFingerprint)) : null,
  ));
}

function vLogin() {
  const err = h('p', {class: 'err', hidden: true});
  const pw = h('input', {id: 'login-pw', type: 'password', autocomplete: 'current-password', required: true, maxlength: 256});
  const btn = h('button', {class: 'b primary', type: 'submit'}, 'Kirjaudu');
  setTimeout(() => pw.focus(), 0);
  return h('div', {class: 'auth'}, h('section', {class: 'panel'},
    h('div', {class: 'brand'}, shield(), 'Vahti'),
    h('h1', {}, 'Kirjaudu sisään'),
    S.session.demo ? h('p', {class: 'note'}, 'Esittelytila on käynnissä.') : null,
    h('form', {class: 'grid', onsubmit: async (e) => {
      e.preventDefault(); err.hidden = true; btn.disabled = true;
      try {
        const r = await api('POST', '/api/login', {password: pw.value});
        S.session = Object.assign(S.session, r); await load();
      } catch (x) { err.textContent = x.message; err.hidden = false; btn.disabled = false; pw.select(); }
    }}, h('label', {class: 'field', for: 'login-pw'}, 'Salasana', pw), err, btn),
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
function verdictPill(e) {
  if (e.queued) return h('span', {class: 'pill hold'}, 'Odottaa');
  return h('span', {class: 'pill ' + e.action}, e.action === 'allow' ? 'Sallittu' : 'Estetty');
}

function vOverview() {
  const o = S.data.o;
  if (!o) return [h('p', {class: 'muted'}, 'Ladataan…')];
  const cats = Object.entries(o.categories || {}).sort((a, b) => b[1] - a[1]);
  const max = Math.max(1, ...cats.map((c) => c[1]));
  const paused = o.pausedUntil && new Date(o.pausedUntil) > new Date();
  return [
    paused ? h('div', {class: 'alert warn'}, h('p', {}, h('b', {}, 'Suojaus on tauolla. '), 'Kaikki liikenne paitsi haitalliset kohteet päästetään läpi klo ' + fmtTime(o.pausedUntil).slice(0, 5) + ' asti.'),
      h('button', {class: 'b', onclick: () => act(() => api('POST', '/api/pause', {minutes: 0}), 'Suojaus jatkuu')}, 'Jatka suojausta')) : null,
    (o.warnings || []).map((w) => h('div', {class: 'alert info'}, h('p', {}, w))),
    h('div', {class: 'stats'},
      stat('Sallittu', fmtNum(o.allowed), 'c-ok', 'kyselyä'),
      stat('Estetty', fmtNum(o.blocked), 'c-bad', 'kyselyä'),
      stat('Odottaa', fmtNum(o.queue), 'c-warn', 'päätöstä'),
      stat('Laitteita', fmtNum(o.devices), '', 'kotiverkossa')),
    o.queue ? h('div', {class: 'qi'}, h('p', {}, h('b', {}, o.queue + ' uutta kohdetta odottaa päätöstäsi. '), 'Ne pysyvät estettyinä, kunnes päätät.'),
      h('div', {class: 'btns'}, h('button', {class: 'b primary', onclick: () => { S.view = 'queue'; render(); load(); }}, 'Käy läpi'))) : null,
    h('div', {class: 'grid2'},
      h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Liikenne nyt'), h('span', {class: 'muted small'}, 'päivittyy 3 s välein')),
        o.recent && o.recent.length ? h('div', {class: 'feed'}, o.recent.map(evRow))
          : h('p', {class: 'muted'}, 'Ei vielä kyselyjä. Kun laitteet käyttävät Vahtia nimipalvelimena, liikenne näkyy tässä.')),
      h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Mitä estettiin'), h('span', {class: 'muted small'}, 'luokittain')),
        cats.length ? h('div', {class: 'catbars'}, cats.map(([k, v]) => h('div', {class: 'catrow'},
          h('span', {}, catName(k)), h('div', {class: 'track'}, h('div', {class: 'fill', style: {width: (v / max * 100) + '%'}})), h('span', {class: 'num'}, fmtNum(v)))))
          : h('p', {class: 'muted'}, 'Ei estoja vielä.'),
        h('p', {class: 'note'}, 'Vahti näkee, mihin palveluun laite yrittää ottaa yhteyttä, mutta ei salatun liikenteen sisältöä. Estot tehdään ennen kuin yhteys avataan, joten estetty pyyntö ei koskaan lähde kotiverkosta.')),
    ),
  ];
}
function stat(label, value, cls, unit) {
  return h('div', {class: 'stat'}, h('span', {class: 'eyebrow'}, label), h('b', {class: cls}, value), h('span', {class: 'muted'}, unit));
}

// ------------------------------------------------------------------ devices
function vDevices() {
  const ds = S.data.devices;
  if (!ds) return [h('p', {class: 'muted'}, 'Ladataan…')];
  if (!ds.length) return [h('div', {class: 'empty'}, h('b', {}, 'Ei vielä laitteita. '), 'Laitteet ilmestyvät tähän, kun ne tekevät ensimmäisen nimikyselyn Vahdin kautta.')];
  return [
    h('div', {class: 'panel-h'}, h('h2', {}, 'Laitteet kotiverkossa'), h('span', {class: 'muted small'}, ds.length + ' laitetta · valitse laite säätääksesi sen oikeuksia')),
    h('div', {class: 'devices'}, ds.map((d) => {
      const tot = (d.allowed + d.blocked) || 1;
      return h('button', {class: 'dev', type: 'button', onclick: () => openSheet(d.id)},
        h('div', {class: 'dev-h'}, h('span', {class: 'ico'}, svgIcon(d.kind)),
          h('div', {}, h('b', {}, d.name), h('div', {class: 'muted small'}, KINDS[d.kind] + ' · ' + MODES[d.mode].n))),
        h('div', {class: 'ratio', 'aria-label': 'Sallittu ' + d.allowed + ', estetty ' + d.blocked},
          h('i', {class: 'a', style: {width: (d.allowed / tot * 100) + '%'}}), h('i', {class: 'b', style: {width: (d.blocked / tot * 100) + '%'}})),
        h('div', {class: 'dev-meta'}, h('span', {}, h('b', {}, fmtNum(d.allowed)), ' sallittu'), h('span', {}, h('b', {}, fmtNum(d.blocked)), ' estetty'),
          h('span', {}, ago(d.lastSeen))));
    })),
  ];
}

async function openSheet(id) {
  S.sheet = id;
  try { S.sheetData = await api('GET', '/api/devices/' + encodeURIComponent(id)); } catch (e) { toast(e.message); S.sheet = null; }
  render();
}
function closeSheet() { S.sheet = null; S.sheetData = null; render(); }
document.addEventListener('keydown', (e) => { if (e.key === 'Escape' && S.sheet) closeSheet(); });

function vSheet() {
  const {device: d, destinations, rules, defaults} = S.sheetData;
  const nameIn = h('input', {id: 'dev-name', value: d.name, maxlength: 60});
  const kindSel = h('select', {id: 'dev-kind', onchange: () => patch({kind: kindSel.value}, 'Tyyppi tallennettu')},
    Object.entries(KINDS).map(([k, v]) => h('option', {value: k, selected: d.kind === k}, v)));
  const patch = (body, msg) => act(() => api('PATCH', '/api/devices/' + encodeURIComponent(d.id), body), msg);
  const learnLeft = d.mode === 'learning' && d.learnUntil ? ' Opettelu päättyy ' + fmtDateTime(d.learnUntil) + '.' : '';
  const ruleFor = (dom) => (rules || []).find((r) => r.domain === dom);
  return [
    h('div', {class: 'sheet-bg', onclick: closeSheet}),
    h('aside', {class: 'sheet', role: 'dialog', 'aria-modal': 'true', 'aria-label': d.name},
      h('div', {class: 'sheet-h'}, h('span', {class: 'ico'}, svgIcon(d.kind)),
        h('div', {}, h('h2', {}, d.name), h('div', {class: 'mono muted'}, d.ip + (d.mac ? ' · ' + d.mac : ''))),
        h('button', {class: 'x', onclick: closeSheet, 'aria-label': 'Sulje'}, '✕')),
      h('section', {class: 'panel'},
        h('form', {class: 'row', onsubmit: (e) => { e.preventDefault(); patch({name: nameIn.value}, 'Nimi tallennettu'); }},
          h('label', {class: 'field', for: 'dev-name'}, 'Nimi', nameIn), h('button', {class: 'b', type: 'submit'}, 'Tallenna')),
        h('label', {class: 'field', for: 'dev-kind'}, 'Laitteen tyyppi', kindSel)),
      h('section', {class: 'panel'}, h('h2', {}, 'Miten laite saa käyttää verkkoa'),
        h('div', {class: 'modes'}, Object.entries(MODES).map(([k, m]) => h('button', {class: 'mode', type: 'button', 'aria-pressed': d.mode === k ? 'true' : 'false',
          onclick: () => patch(k === 'learning' ? {mode: k, learnHours: 48} : {mode: k}, 'Tila: ' + m.n)},
          h('b', {}, m.n), h('p', {}, m.d + (d.mode === k ? learnLeft : '')))))),
      h('section', {class: 'panel'}, h('h2', {}, 'Mitä tämä laite saa jakaa'),
        h('div', {class: 'rules'}, CAT_ORDER.filter((c) => c !== 'threat').map((c) => {
          const ov = (d.catOverride || {})[c] || '';
          return h('div', {class: 'rule'}, h('div', {}, h('h3', {}, CATS[c].n), h('p', {}, 'Oletus: ' + (defaults[c] === 'allow' ? 'sallittu' : 'estetty'))),
            seg(ov, [['', 'Oletus', 'd'], ['allow', 'Salli', 'a'], ['block', 'Estä', 'x']], (v) => {
              const body = {catOverride: {}}; body.catOverride[c] = v || null;
              patch(body, CATS[c].n + ': ' + (v === 'allow' ? 'sallittu' : v === 'block' ? 'estetty' : 'oletus'));
            }));
        }))),
      h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Kohteet, joihin laite on ottanut yhteyttä'), h('span', {class: 'muted small'}, 'viimeisimmistä kyselyistä')),
        destinations.length ? h('div', {class: 'rules'}, destinations.map((x) => {
          const r = ruleFor(x.domain);
          return h('div', {class: 'rule'},
            h('div', {}, h('div', {class: 'mono'}, x.domain),
              h('p', {}, [catName(x.category), x.owner, fmtNum(x.allowed) + ' sallittu', fmtNum(x.blocked) + ' estetty'].filter(Boolean).join(' · '),
                r ? ' · ' + (r.action === 'allow' ? 'oma lupa' : 'oma esto') + (r.until ? ' klo ' + fmtTime(r.until).slice(0, 5) + ' asti' : '') : '')),
            x.category === 'threat' ? null : h('div', {class: 'btns'},
              x.blocked ? h('button', {class: 'b small', onclick: () => act(() => api('POST', '/api/rules', {device: d.id, domain: x.domain, action: 'allow', hours: 1}), 'Jaettu tunnin ajan')}, 'Jaa tunniksi') : null,
              x.allowed ? h('button', {class: 'b small danger', onclick: () => act(() => api('POST', '/api/rules', {device: d.id, domain: x.domain, action: 'block', hours: 0}), 'Estetty tältä laitteelta')}, 'Estä') : null));
        })) : h('p', {class: 'muted'}, 'Ei liikennettä viimeisimmissä kyselyissä.')),
      (rules || []).length ? h('section', {class: 'panel'}, h('h2', {}, 'Laitteen omat säännöt'), rulesList(rules)) : null,
      h('section', {class: 'panel'}, h('h2', {}, 'Unohda laite'), h('p', {class: 'muted small'}, 'Poistaa laitteen, sen säännöt ja odottavat päätökset. Laite palaa listalle uutena, jos se käyttää verkkoa uudelleen.'),
        armedButton('Unohda laite', 'Vahvista: unohda laite', async () => {
          try { await api('DELETE', '/api/devices/' + encodeURIComponent(d.id)); toast('Laite unohdettu'); closeSheet(); load(); } catch (e) { toast(e.message); }
        })),
    ),
  ];
}

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
  if (!q) return [h('p', {class: 'muted'}, 'Ladataan…')];
  if (!q.length) return [h('div', {class: 'empty'}, h('b', {}, 'Ei odottavia päätöksiä. '),
    'Kun lukittu laite yrittää ottaa yhteyttä uuteen kohteeseen, se näkyy tässä ja pysyy estettynä, kunnes päätät. Lukitse älylaitteet Laitteet-näkymässä.')];
  return [
    h('div', {class: 'panel-h'}, h('h2', {}, 'Odottaa päätöstä'), h('span', {class: 'muted small'}, 'Kaikki alla olevat ovat estettyinä, kunnes sallit ne.')),
    h('div', {class: 'q'}, q.map(qCard)),
  ];
}

function qCard(it) {
  const siteId = 'site-' + it.id;
  const site = h('input', {type: 'checkbox', id: siteId});
  const decide = (decision, msg) => act(() => api('POST', '/api/queue/' + encodeURIComponent(it.id), {decision, scope: site.checked ? 'site' : 'name'}), msg);
  const facts = [
    ['Omistaja', it.owner ? h('b', {}, it.owner) : h('span', {class: 'muted'}, 'Ei tiedossa paikallisessa tietokannassa')],
    ['Sivusto', h('span', {class: 'mono'}, it.registrable)],
    it.category ? ['Luokka', [catName(it.category), it.listSource ? h('span', {class: 'muted'}, ' · lähde: ' + it.listSource) : null]] : null,
    it.note ? ['Mikä se on', it.note] : null,
    it.siblings && it.siblings.length ? ['Saman sivuston muut osoitteet', h('ul', {}, it.siblings.map((s) => h('li', {class: 'mono'}, s)))] : null,
    ['Muut laitteet', it.otherDevices && it.otherDevices.length ? it.otherDevices.join(', ') + ' on myös ottanut yhteyttä tähän kohteeseen' : 'Vain tämä laite'],
    it.vias && it.vias.length ? ['Ohjautuu osoitteeseen', it.vias.join(', ')] : null,
    ['Yritykset', it.count + ' kertaa · ensimmäinen ' + fmtDateTime(it.first) + ' · viimeisin ' + ago(it.last)],
  ].filter(Boolean);
  return h('div', {class: 'qi'},
    h('div', {class: 'dev-h'}, h('span', {class: 'ico'}, svgIcon(it.deviceKind)),
      h('div', {}, h('b', {}, it.deviceName), ' yrittää ottaa yhteyttä kohteeseen', h('div', {class: 'mono'}, it.domain))),
    h('div', {class: 'intel'}, h('span', {class: 'eyebrow'}, 'Mihin kohde liittyy'),
      h('dl', {}, facts.map(([k, v]) => [h('dt', {}, k), h('dd', {}, v)])),
      h('p', {class: 'muted small'}, 'Tiedot tulevat Vahti-laitteen omasta tietokannasta ja kotiverkon liikenteestä. Kohteesta ei kysytä mitään internetistä.')),
    h('label', {class: 'check', for: siteId}, site, 'Päätös koskee koko sivustoa ', h('span', {class: 'mono'}, it.registrable)),
    h('div', {class: 'btns'},
      h('button', {class: 'b', onclick: () => decide('once', 'Sallittu kerran')}, 'Salli kerran'),
      h('button', {class: 'b', onclick: () => decide('hour', 'Sallittu tunnin ajan')}, 'Salli tunniksi'),
      h('button', {class: 'b', onclick: () => decide('allow', 'Sallittu pysyvästi')}, 'Salli aina'),
      h('button', {class: 'b primary', onclick: () => decide('block', 'Estetty pysyvästi')}, 'Estä aina')));
}

// ------------------------------------------------------------------ rules
function rulesList(rules) {
  return h('div', {class: 'rules'}, rules.map((r) => h('div', {class: 'rule'},
    h('div', {}, h('div', {}, h('span', {class: 'mono'}, r.domain), ' ', h('span', {class: 'pill ' + r.action}, r.action === 'allow' ? 'Sallittu' : 'Estetty')),
      h('p', {}, [r.deviceName,
        r.once ? 'seuraavan kerran' : r.until ? 'klo ' + fmtDateTime(r.until) + ' asti' : 'pysyvä',
        {user: 'oma sääntö', queue: 'päätös jonosta', learned: 'opittu', share: 'jaettu'}[r.source] || r.source].join(' · '))),
    h('button', {class: 'b small danger', onclick: () => act(() => api('DELETE', '/api/rules/' + encodeURIComponent(r.id)), 'Sääntö poistettu')}, 'Poista'))));
}

function vRules() {
  const {settings: st, rules, devices} = S.data;
  if (!st || !rules) return [h('p', {class: 'muted'}, 'Ladataan…')];
  const domIn = h('input', {id: 'rule-domain', placeholder: 'esim. tracker.example.com', maxlength: 253, required: true});
  const devSel = h('select', {id: 'rule-device'}, h('option', {value: '*'}, 'Kaikki laitteet'), (devices || []).map((d) => h('option', {value: d.id}, d.name)));
  const actSel = h('select', {id: 'rule-action'}, h('option', {value: 'block'}, 'Estä'), h('option', {value: 'allow'}, 'Salli'));
  const userRules = rules.filter((r) => r.source !== 'learned');
  const learned = rules.filter((r) => r.source === 'learned');
  return [h('div', {class: 'grid2'},
    h('section', {class: 'panel'}, h('h2', {}, 'Oletukset kaikille laitteille'),
      h('div', {class: 'rules'}, CAT_ORDER.map((c) => h('div', {class: 'rule'},
        h('div', {}, h('h3', {}, CATS[c].n), h('p', {}, CATS[c].d),
          CATS[c].c && st.categories[c] === 'block' ? h('p', {class: 'warnnote'}, CATS[c].c) : null,
          CATS[c].locked ? h('p', {}, 'Aina estetty.') : null),
        seg(st.categories[c], [['allow', 'Salli', 'a'], ['block', 'Estä', 'x']], (v) => {
          const cats = {}; cats[c] = v;
          act(() => api('PUT', '/api/settings', {categories: cats}), CATS[c].n + ': ' + (v === 'allow' ? 'sallittu' : 'estetty'));
        }, CATS[c].locked))))),
    h('div', {class: 'stack'},
      h('section', {class: 'panel'}, h('h2', {}, 'Uudet laitteet'),
        h('p', {class: 'muted small'}, 'Missä tilassa uusi laite aloittaa, kun se ilmestyy verkkoon.'),
        seg(st.newDeviceMode, Object.entries(MODES).map(([k, m]) => [k, m.n, 'd']), (v) => act(() => api('PUT', '/api/settings', {newDeviceMode: v}), 'Uudet laitteet: ' + MODES[v].n))),
      h('section', {class: 'panel'}, h('h2', {}, 'Lisää oma sääntö'),
        h('form', {class: 'grid', onsubmit: (e) => {
          e.preventDefault();
          act(() => api('POST', '/api/rules', {device: devSel.value, domain: domIn.value.trim(), action: actSel.value, hours: 0}), 'Sääntö lisätty').then(() => { domIn.value = ''; });
        }},
        h('label', {class: 'field', for: 'rule-domain'}, 'Verkkotunnus (koskee myös alaosoitteita)', domIn),
        h('div', {class: 'row'}, h('label', {class: 'field', for: 'rule-device'}, 'Laite', devSel), h('label', {class: 'field', for: 'rule-action'}, 'Toiminto', actSel),
          h('button', {class: 'b primary', type: 'submit'}, 'Lisää')))),
      h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Omat säännöt ja luvat'), h('span', {class: 'muted small'}, String(userRules.length))),
        userRules.length ? rulesList(userRules) : h('p', {class: 'muted'}, 'Säännöt syntyvät, kun teet päätöksen odottavasta kohteesta, jaat dataa laitteen sivulta tai lisäät säännön itse.')),
      learned.length ? h('section', {class: 'panel'}, h('div', {class: 'panel-h'}, h('h2', {}, 'Opitut kohteet'), h('span', {class: 'muted small'}, String(learned.length))), rulesList(learned)) : null,
    ))];
}

// ------------------------------------------------------------------ log
function vLog() {
  const f = S.logFilter;
  const rows = S.data.log;
  const search = h('input', {id: 'log-q', type: 'search', placeholder: 'Hae verkkotunnusta', value: f.q, maxlength: 100,
    oninput: () => { f.q = search.value.trim(); clearTimeout(vLog.t); vLog.t = setTimeout(load, 300); }});
  const devSel = h('select', {id: 'log-device', 'aria-label': 'Laite', onchange: () => { f.device = devSel.value; load(); }},
    h('option', {value: ''}, 'Kaikki laitteet'), (S.data.devices || []).map((d) => h('option', {value: d.id, selected: f.device === d.id}, d.name)));
  return [h('section', {class: 'panel'},
    h('div', {class: 'panel-h'}, h('h2', {}, 'Loki'),
      h('div', {class: 'filters'},
        h('div', {class: 'chips'}, [['all', 'Kaikki'], ['allow', 'Sallitut'], ['block', 'Estetyt']].map(([v, l]) =>
          h('button', {type: 'button', 'aria-pressed': f.action === v ? 'true' : 'false', onclick: () => { f.action = v; load(); }}, l))),
        devSel, search)),
    !rows ? h('p', {class: 'muted'}, 'Ladataan…') : !rows.length ? h('p', {class: 'muted'}, 'Ei osumia.') :
      h('div', {class: 'tablewrap'}, h('table', {},
        h('thead', {}, h('tr', {}, ['Aika', 'Laite', 'Kohde', 'Tyyppi', 'Luokka', 'Päätös', 'Peruste'].map((t) => h('th', {}, t)))),
        h('tbody', {}, rows.map((e) => h('tr', {},
          h('td', {class: 'mono'}, fmtTime(e.time)), h('td', {}, e.deviceName),
          h('td', {class: 'mono'}, e.domain, e.via ? h('div', {class: 'muted'}, '→ ' + e.via) : null),
          h('td', {class: 'mono muted'}, e.qtype),
          h('td', {}, h('span', {class: 'chip'}, e.category ? catName(e.category) : (e.owner || 'Tuntematon'))),
          h('td', {}, verdictPill(e)), h('td', {class: 'muted'}, e.reason)))))),
    h('p', {class: 'note'}, 'Loki säilyy vain Vahti-laitteen muistissa, eikä sitä kirjoiteta levylle tai lähetetä minnekään. Uudelleenkäynnistys tyhjentää sen.'),
  )];
}

// ------------------------------------------------------------------ settings
function vSettings() {
  const st = S.data.settings;
  if (!st) return [h('p', {class: 'muted'}, 'Ladataan…')];
  const paused = st.pausedUntil && new Date(st.pausedUntil) > new Date();
  const cur = h('input', {id: 'pw-cur', type: 'password', autocomplete: 'current-password', required: true, maxlength: 256});
  const nw = h('input', {id: 'pw-new', type: 'password', autocomplete: 'new-password', required: true, minlength: 12, maxlength: 256});
  const nw2 = h('input', {id: 'pw-new2', type: 'password', autocomplete: 'new-password', required: true, maxlength: 256});
  const pause = (m, msg) => act(() => api('POST', '/api/pause', {minutes: m}), msg);
  return [h('div', {class: 'grid2'},
    h('div', {class: 'stack'},
      h('section', {class: 'panel'}, h('h2', {}, 'Tauko'),
        h('p', {class: 'muted small'}, 'Jos jokin palvelu ei toimi, voit keskeyttää suodatuksen hetkeksi. Haitalliset kohteet estetään myös tauon aikana.'),
        paused ? h('div', {class: 'btns'}, h('span', {}, 'Tauolla klo ' + fmtTime(st.pausedUntil).slice(0, 5) + ' asti'), h('button', {class: 'b primary', onclick: () => pause(0, 'Suojaus jatkuu')}, 'Jatka suojausta'))
          : h('div', {class: 'btns'}, h('button', {class: 'b', onclick: () => pause(15, 'Tauko 15 minuuttia')}, '15 min'), h('button', {class: 'b', onclick: () => pause(60, 'Tauko tunnin')}, '1 tunti'))),
      h('section', {class: 'panel'}, h('h2', {}, 'Vaihda salasana'),
        h('form', {class: 'grid', onsubmit: (e) => {
          e.preventDefault();
          if (nw.value !== nw2.value) { toast('Uudet salasanat eivät täsmää.'); return; }
          act(() => api('POST', '/api/password', {current: cur.value, next: nw.value}), 'Salasana vaihdettu. Muut istunnot kirjattiin ulos.').then(() => { cur.value = nw.value = nw2.value = ''; });
        }},
        h('label', {class: 'field', for: 'pw-cur'}, 'Nykyinen salasana', cur),
        h('label', {class: 'field', for: 'pw-new'}, 'Uusi salasana (vähintään 12 merkkiä)', nw),
        h('label', {class: 'field', for: 'pw-new2'}, 'Uusi salasana uudelleen', nw2),
        h('div', {class: 'btns'}, h('button', {class: 'b primary', type: 'submit'}, 'Vaihda salasana')))),
      h('section', {class: 'panel'}, h('h2', {}, 'Kirjaudu ulos'),
        h('div', {class: 'btns'}, h('button', {class: 'b', onclick: async () => {
          try { await api('POST', '/api/logout', {}); } catch (e) { /* ignore */ }
          S.session.authenticated = false; S.session.csrf = null; S.data = {}; render();
        }}, 'Kirjaudu ulos'))),
    ),
    h('div', {class: 'stack'},
      h('section', {class: 'panel'}, h('h2', {}, 'Tunnistetietokannat'),
        h('p', {class: 'muted small'}, 'Listat ladataan vain, kun pyydät sitä (komento update-lists). Muuten Vahti ei ota yhteyttä mihinkään omasta aloitteestaan.'),
        h('div', {class: 'rules'}, (st.lists || []).map((l) => h('div', {class: 'rule'},
          h('div', {}, h('h3', {}, l.name === 'Vahti' ? 'Sisäänrakennettu lista' : l.name), h('p', {}, l.error ? l.error : fmtNum(l.entries) + ' verkkotunnusta' + (l.category !== 'mixed' ? ' · ' + catName(l.category) : ''))),
          h('span', {class: 'pill ' + (l.error ? 'hold' : 'allow')}, l.error ? 'Ei ladattu' : 'Käytössä'))))),
      h('section', {class: 'panel'}, h('h2', {}, 'Nimipalvelut'),
        h('p', {class: 'muted small'}, 'Sallitut kyselyt välitetään näille palvelimille salattuna. Estetyt kyselyt eivät lähde kotiverkosta.'),
        h('ul', {class: 'mono'}, (st.upstreams || []).map((u) => h('li', {}, u))),
        h('p', {class: 'muted small'}, 'Versio ' + (st.version || '?'))),
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
