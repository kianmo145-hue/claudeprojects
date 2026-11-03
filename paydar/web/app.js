'use strict';
// Paydar UI. Talks to the Go core over /api/* with the token from the URL fragment. On Android
// the page also has window.PaydarNative for things only the app can do (VPN permission, clipboard).

const T = {
  fa: {
    appName: 'پایدار', tabServers: 'سرورها', tabSubs: 'اشتراک‌ها', tabActivity: 'رویدادها', tabSettings: 'تنظیمات',
    add: 'افزودن', paste: 'چسباندن از کلیپ‌بورد', testAll: 'تست همه', removeDead: 'حذف خراب‌ها', autoPick: 'انتخاب خودکار',
    addSub: 'افزودن اشتراک', refreshAll: 'به‌روزرسانی همه', switches: 'سوییچ‌ها و قطعی‌ها', logs: 'گزارش‌ها',
    emptyServers: 'هنوز سروری ندارید. لینک کانفیگ یا اشتراک را کپی کنید و «چسباندن» را بزنید.',
    emptySubs: 'لینک اشتراک (subscription) را بالا وارد کنید. هر چند ساعت خودکار به‌روز می‌شود.',
    addTitle: 'افزودن کانفیگ', addHelp: 'یک یا چند لینک (هر خط یکی)، متن base64، کانفیگ JSON سینگ‌باکس یا لینک اشتراک.',
    cancel: 'انصراف', save: 'ذخیره', quit: 'خروج کامل از برنامه',
    disconnected: 'قطع', connecting: 'در حال اتصال…', connected: 'متصل', tapToConnect: 'برای اتصال بزنید',
    auto: 'سوییچ خودکار', manual: 'دستی', chosen: 'انتخاب شما', current: 'فعال', ms: 'ms',
    offline: 'اینترنت شبکهٔ شما قطع است؛ به محض وصل شدن، اتصال برمی‌گردد.',
    noServer: 'هیچ سروری جواب نمی‌دهد؛ مدام در حال تلاش برای پیدا کردن سرور سالم…',
    testing: 'در حال تست…', untested: 'تست نشده', down: 'قطع', success: 'موفقیت',
    copyLink: 'کپی لینک', del: 'حذف', update: 'به‌روزرسانی', servers: 'سرور', updated: 'به‌روز شده',
    used: 'مصرف', of: 'از', daysLeft: 'روز مانده', expired: 'منقضی', unlimited: 'نامحدود',
    added: n => `${n} کانفیگ اضافه شد`, subAdded: n => `اشتراک با ${n} سرور اضافه شد`, removed: n => `${n} سرور حذف شد`,
    copied: 'کپی شد', saved: 'ذخیره شد', confirmDel: 'حذف شود؟', switched: (a, b) => `${a} ← ${b}`,
    uptime: 'مدت اتصال', mode_tun: 'کل سیستم (TUN)', mode_proxy: 'پراکسی سیستم', mode_local: 'VPN',
    reason_manual: 'انتخاب دستی', 'reason_server stopped responding': 'سرور قبلی جواب نداد',
    'reason_a much faster server is available': 'سرور خیلی سریع‌تر پیدا شد', 'reason_back to your chosen server': 'بازگشت به سرور انتخابی شما',
    'reason_no server is responding': 'هیچ سروری جواب نداد', 'reason_the local network has no internet': 'اینترنت شبکه قطع شد',
    'reason_internet is back': 'اینترنت برگشت', 'reason_server not responding (auto-switch is off)': 'سرور جواب نمی‌دهد (سوییچ خودکار خاموش است)',
    sAuto: 'سوییچ خودکار بین کانفیگ‌ها', sAutoHelp: 'اگر سرور فعال جواب ندهد، در چند ثانیه به سالم‌ترین سرور منتقل می‌شوید.',
    sFaster: 'ترجیح سرور سریع‌تر', sFasterHelp: 'وقتی سروری خیلی سریع‌تر (حداقل دو برابر) و پایدار باشد، با فاصلهٔ زمانی کافی به آن می‌رود.',
    sIran: 'سایت‌های ایرانی مستقیم', sIranHelp: 'دامنه‌های .ir بدون VPN باز شوند (سریع‌تر و کم‌مصرف‌تر).',
    sIPv6: 'IPv6', sIPv6Help: 'فقط اگر سرورها و شبکهٔ شما IPv6 دارند روشن کنید.',
    sAutoConnect: 'اتصال خودکار هنگام اجرا', sAutoConnectHelp: 'با باز شدن برنامه خودش وصل شود.',
    sMode: 'حالت اتصال', modeTun: 'کل سیستم (TUN، پیشنهادی)', modeProxy: 'فقط پراکسی سیستم',
    sDNS: 'DNS از طریق تونل', sPort: 'پورت پراکسی محلی (SOCKS/HTTP)', sSubHours: 'به‌روزرسانی اشتراک هر چند ساعت (۰ = خودکار)',
    version: v => `نسخهٔ ${v}`,
  },
  en: {
    appName: 'Paydar', tabServers: 'Servers', tabSubs: 'Subscriptions', tabActivity: 'Activity', tabSettings: 'Settings',
    add: 'Add', paste: 'Paste from clipboard', testAll: 'Test all', removeDead: 'Remove dead', autoPick: 'Pick automatically',
    addSub: 'Add subscription', refreshAll: 'Update all', switches: 'Switches and outages', logs: 'Logs',
    emptyServers: 'No servers yet. Copy a config or subscription link and press “Paste”.',
    emptySubs: 'Enter a subscription link above. It updates itself every few hours.',
    addTitle: 'Add configs', addHelp: 'One or more links (one per line), base64 text, a sing-box JSON config, or a subscription URL.',
    cancel: 'Cancel', save: 'Save', quit: 'Quit Paydar',
    disconnected: 'Disconnected', connecting: 'Connecting…', connected: 'Connected', tapToConnect: 'Tap to connect',
    auto: 'Auto-switch', manual: 'Manual', chosen: 'Your pick', current: 'Active', ms: 'ms',
    offline: 'Your network has no internet; the connection resumes as soon as it is back.',
    noServer: 'No server is answering; still looking for a working one…',
    testing: 'Testing…', untested: 'Not tested', down: 'Down', success: 'success',
    copyLink: 'Copy link', del: 'Delete', update: 'Update', servers: 'servers', updated: 'updated',
    used: 'Used', of: 'of', daysLeft: 'days left', expired: 'Expired', unlimited: 'Unlimited',
    added: n => `${n} config(s) added`, subAdded: n => `Subscription added with ${n} servers`, removed: n => `${n} server(s) removed`,
    copied: 'Copied', saved: 'Saved', confirmDel: 'Delete?', switched: (a, b) => `${a} → ${b}`,
    uptime: 'Connected for', mode_tun: 'Whole system (TUN)', mode_proxy: 'System proxy', mode_local: 'VPN',
    sAuto: 'Switch between configs automatically', sAutoHelp: 'If the active server stops answering, traffic moves to the healthiest one within seconds.',
    sFaster: 'Prefer faster servers', sFasterHelp: 'Moves to a server that is at least twice as fast and stable, never more than once per 10 minutes.',
    sIran: 'Iranian sites direct', sIranHelp: '.ir domains bypass the VPN (faster, less traffic).',
    sIPv6: 'IPv6', sIPv6Help: 'Only if your servers and network have IPv6.',
    sAutoConnect: 'Connect on start', sAutoConnectHelp: 'Connects as soon as the app opens.',
    sMode: 'Connection mode', modeTun: 'Whole system (TUN, recommended)', modeProxy: 'System proxy only',
    sDNS: 'DNS through the tunnel', sPort: 'Local proxy port (SOCKS/HTTP)', sSubHours: 'Update subscriptions every N hours (0 = auto)',
    version: v => `Version ${v}`,
  },
};

const $ = s => document.querySelector(s);
const token = (() => {
  const m = location.hash.match(/t=([0-9a-f]+)/);
  try {
    if (m) sessionStorage.setItem('paydarToken', m[1]);
    return m ? m[1] : sessionStorage.getItem('paydarToken');
  } catch (e) { return m && m[1]; }
})();
const native = window.PaydarNative || null;
let lang = 'fa';
let platform = '';
let status = {};
let servers = [];
let settings = {};

function t(k, ...a) {
  const v = (T[lang][k] !== undefined ? T[lang][k] : T.en[k]);
  if (v === undefined) return k;
  return typeof v === 'function' ? v(...a) : v;
}

async function api(method, args) {
  const r = await fetch('/api/' + method, {
    method: 'POST', headers: { 'Content-Type': 'application/json', 'X-Token': token || '' },
    body: JSON.stringify(args || {}),
  });
  if (!r.ok) throw new Error(r.status === 401 ? 'session expired, reopen the app' : 'HTTP ' + r.status);
  const j = await r.json();
  if (!j.ok) throw new Error(j.error || 'error');
  return j.data;
}

let toastTimer;
function toast(msg, err) {
  const el = $('#toast');
  el.textContent = msg;
  el.className = 'toast' + (err ? ' err' : '');
  clearTimeout(toastTimer);
  toastTimer = setTimeout(() => el.classList.add('hidden'), 3500);
}
const run = fn => async (...a) => { try { await fn(...a); } catch (e) { toast(e.message, true); } };

function applyLang() {
  document.documentElement.lang = lang;
  document.documentElement.dir = lang === 'fa' ? 'rtl' : 'ltr';
  document.querySelectorAll('[data-t]').forEach(el => { el.textContent = t(el.dataset.t); });
  $('#langBtn').textContent = lang === 'fa' ? 'EN' : 'فا';
  document.title = t('appName');
  render();
}

function el(tag, cls, text) {
  const e = document.createElement(tag);
  if (cls) e.className = cls;
  if (text !== undefined) e.textContent = text;
  return e;
}

function fmtDur(ms) {
  const s = Math.floor(ms / 1000), h = Math.floor(s / 3600), m = Math.floor(s % 3600 / 60);
  return h ? `${h}:${String(m).padStart(2, '0')}:${String(s % 60).padStart(2, '0')}` : `${m}:${String(s % 60).padStart(2, '0')}`;
}
function fmtBytes(n) {
  if (!n) return '0';
  const u = ['B', 'KB', 'MB', 'GB', 'TB']; let i = 0;
  while (n >= 1024 && i < u.length - 1) { n /= 1024; i++; }
  return n.toFixed(i > 1 ? 1 : 0) + ' ' + u[i];
}
function fmtTime(ms) {
  return new Date(ms).toLocaleTimeString(lang === 'fa' ? 'fa-IR' : 'en-GB', { hour: '2-digit', minute: '2-digit', second: '2-digit' });
}
function nameOf(tagOrId) {
  const id = (tagOrId || '').replace(/^p-/, '');
  const s = servers.find(x => x.id === id);
  return s ? s.name : id;
}
function health(s) {
  const h = s.health;
  if (h) return h;
  if (s.lastTested) return { state: s.lastOkResult ? 'ok' : 'down', rtt: s.lastRtt, samples: 1, success: s.lastOkResult ? 1 : 0, stale: true };
  return { state: 'unknown' };
}

function renderHero() {
  const hero = $('#hero');
  hero.className = 'hero ' + (status.state || 'disconnected');
  $('#stateText').textContent = t(status.state || 'disconnected');
  const chips = $('#chips');
  chips.replaceChildren();
  const banner = $('#banner');
  banner.classList.add('hidden');
  if (status.state === 'connected') {
    const cur = servers.find(s => s.id === status.current);
    $('#serverText').textContent = status.serverName || '';
    const h = cur ? health(cur) : null;
    if (h && h.rtt) chips.append(el('span', 'chip ' + (h.state === 'ok' ? 'good' : 'warn'), `${h.rtt} ${t('ms')}`));
    chips.append(el('span', 'chip', t(settings.autoSwitch ? 'auto' : 'manual')));
    if (status.mode) chips.append(el('span', 'chip', t('mode_' + status.mode)));
    if (status.since) chips.append(el('span', 'chip', `${t('uptime')} ${fmtDur(Date.now() - status.since)}`));
    const last = (status.events || []).slice(-1)[0];
    if (status.offline) { banner.textContent = t('offline'); banner.className = 'banner'; }
    else if (last && last.kind === 'down' && Date.now() - last.time < 60000) { banner.textContent = t('noServer'); banner.className = 'banner err'; }
  } else {
    $('#serverText').textContent = status.state === 'connecting' ? '' : t('tapToConnect');
    if (status.error) { banner.textContent = status.error; banner.className = 'banner err'; }
  }
}

function renderServers() {
  const list = $('#servers');
  list.replaceChildren();
  $('#serversEmpty').classList.toggle('hidden', servers.length > 0);
  $('#autoPickBtn').classList.toggle('hidden', !servers.some(s => s.chosen));
  for (const s of servers) {
    const h = health(s);
    const item = el('div', 'item' + (s.current ? ' current' : ''));
    const state = status.testing && !s.health ? 'testing' : h.state;
    item.append(el('span', 'dot ' + state));
    const main = el('div', 'main');
    main.append(el('div', 'name', s.name));
    const meta = el('div', 'meta');
    meta.append(el('span', 'tag', s.type));
    if (s.current) meta.append(el('span', '', '● ' + t('current')));
    if (s.chosen) meta.append(el('span', '', '📌 ' + t('chosen')));
    if (s.subName) meta.append(el('span', '', s.subName));
    if (h.samples > 1) meta.append(el('span', '', `${Math.round(h.success * 100)}% ${t('success')}`));
    main.append(meta);
    item.append(main);
    let ptxt = t('untested'), pcls = '';
    if (state === 'testing') ptxt = t('testing');
    else if (h.state === 'down') { ptxt = t('down'); pcls = 'down'; }
    else if (h.rtt) { ptxt = `${h.rtt} ${t('ms')}`; pcls = h.state; }
    item.append(el('span', 'ping ' + pcls, ptxt));
    const acts = el('div', 'acts');
    const copy = el('button', '', t('copyLink'));
    copy.type = 'button';
    copy.onclick = run(async ev => { ev.stopPropagation(); await copyText(await api('link', { id: s.id })); toast(t('copied')); });
    const del = el('button', 'danger', t('del'));
    del.type = 'button';
    del.onclick = run(async ev => { ev.stopPropagation(); if (confirm(t('confirmDel'))) { await api('remove', { id: s.id }); refresh(); } });
    acts.append(copy, del);
    item.append(acts);
    item.onclick = run(async () => { await api('choose', { id: s.id }); refresh(); });
    list.append(item);
  }
}

let subs = [];
function renderSubs() {
  const list = $('#subs');
  list.replaceChildren();
  $('#subsEmpty').classList.toggle('hidden', subs.length > 0);
  for (const s of subs) {
    const item = el('div', 'item');
    item.style.cursor = 'default';
    const main = el('div', 'main');
    main.append(el('div', 'name', s.name || s.url));
    const meta = el('div', 'meta');
    meta.append(el('span', '', `${s.count} ${t('servers')}`));
    if (s.updatedAt) meta.append(el('span', '', `${t('updated')} ${fmtTime(s.updatedAt)}`));
    if (s.expire) {
      const days = Math.floor((s.expire * 1000 - Date.now()) / 86400000);
      meta.append(el('span', days < 0 ? 'ping down' : '', days < 0 ? t('expired') : `${days} ${t('daysLeft')}`));
    }
    if (s.lastError) meta.append(el('span', 'ping down', s.lastError));
    main.append(meta);
    if (s.total || s.download) {
      const used = (s.upload || 0) + (s.download || 0);
      main.append(el('div', 'meta', `${t('used')} ${fmtBytes(used)} ${t('of')} ${s.total ? fmtBytes(s.total) : t('unlimited')}`));
      if (s.total) {
        const bar = el('div', 'bar'); const fill = el('i');
        fill.style.width = Math.min(100, used / s.total * 100) + '%';
        bar.append(fill); main.append(bar);
      }
    }
    item.append(main);
    const acts = el('div', 'acts');
    acts.style.display = 'flex';
    const up = el('button', '', t('update'));
    up.type = 'button';
    up.onclick = run(async () => { up.disabled = true; try { await api('updateSub', { id: s.id }); } finally { up.disabled = false; refresh(); } });
    const del = el('button', 'danger', t('del'));
    del.type = 'button';
    del.onclick = run(async () => { if (confirm(t('confirmDel'))) { await api('removeSub', { id: s.id }); refresh(); } });
    acts.append(up, del);
    item.append(acts);
    list.append(item);
  }
}

function renderActivity(logs) {
  const ev = $('#events');
  ev.replaceChildren();
  for (const e of (status.events || []).slice().reverse()) {
    const row = el('div', 'event');
    row.append(el('time', '', fmtTime(e.time)));
    let text = e.kind === 'switch' ? t('switched', nameOf(e.from), nameOf(e.to)) : (nameOf(e.from) || '');
    text += ' · ' + t('reason_' + e.reason);
    row.append(document.createTextNode(text.replace(/^ · /, '')));
    ev.append(row);
  }
  if (logs) $('#logs').textContent = logs.map(l => `${new Date(l.time).toLocaleTimeString('en-GB')} ${l.level.toUpperCase().padEnd(5)} ${l.text}`).join('\n');
}

function renderSettings() {
  const f = $('#settingsForm');
  for (const [k, v] of Object.entries(settings)) {
    const input = f.elements[k];
    if (!input) continue;
    if (input.type === 'checkbox') input.checked = !!v; else input.value = v;
  }
}

function render() { renderHero(); renderServers(); renderSubs(); renderActivity(); }

let tab = 'servers';
async function refresh() {
  try {
    const [st, sv] = await Promise.all([api('status'), api('servers')]);
    status = st; servers = sv;
    if (tab === 'subs') subs = await api('subs');
    render();
    if (tab === 'activity') renderActivity(await api('logs'));
  } catch (e) { /* the core may be restarting; try again next tick */ }
}

async function copyText(s) {
  if (native && native.copy) { native.copy(s); return; }
  await navigator.clipboard.writeText(s);
}
async function readClipboard() {
  if (native && native.readClipboard) return native.readClipboard();
  return navigator.clipboard.readText();
}
async function addText(text) {
  const r = await api('add', { text });
  toast(r.sub ? t('subAdded', r.added) : t('added', r.added));
  refresh();
}

function wire() {
  $('#langBtn').onclick = run(async () => {
    lang = lang === 'fa' ? 'en' : 'fa';
    settings.lang = lang;
    applyLang();
    settings = await api('saveSettings', { settings });
  });
  $('#power').onclick = run(async () => {
    if (status.state === 'disconnected') {
      status.state = 'connecting'; renderHero();
      await api('connect');
    } else {
      await api('disconnect');
    }
    setTimeout(refresh, 300);
  });
  document.querySelectorAll('#tabs button').forEach(b => b.onclick = () => {
    tab = b.dataset.tab;
    document.querySelectorAll('#tabs button').forEach(x => x.classList.toggle('on', x === b));
    document.querySelectorAll('.pane').forEach(p => p.classList.toggle('hidden', p.id !== 'pane-' + tab));
    if (tab === 'settings') renderSettings();
    refresh();
  });
  $('#addBtn').onclick = () => { $('#addText').value = ''; $('#addDialog').showModal(); };
  $('#addCancel').onclick = () => $('#addDialog').close();
  $('#addOk').onclick = run(async () => {
    const text = $('#addText').value.trim();
    $('#addDialog').close();
    if (text) await addText(text);
  });
  $('#pasteBtn').onclick = run(async () => {
    const text = (await readClipboard() || '').trim();
    if (!text) { $('#addDialog').showModal(); return; }
    await addText(text);
  });
  $('#testBtn').onclick = run(async () => { await api('test'); status.testing = true; renderServers(); });
  $('#deadBtn').onclick = run(async () => { const n = await api('removeDead'); toast(t('removed', n || 0)); refresh(); });
  $('#autoPickBtn').onclick = run(async () => { await api('clearChoice'); refresh(); });
  $('#subForm').onsubmit = run(async ev => {
    ev.preventDefault();
    const url = $('#subUrl').value.trim();
    const n = await api('addSub', { url });
    $('#subUrl').value = '';
    toast(t('subAdded', n));
    refresh();
  });
  $('#subsRefresh').onclick = run(async () => { await api('updateSubs'); setTimeout(refresh, 3000); });
  $('#settingsForm').onsubmit = run(async ev => {
    ev.preventDefault();
    const f = ev.target, s = { ...settings };
    for (const input of f.elements) {
      if (!input.name) continue;
      s[input.name] = input.type === 'checkbox' ? input.checked : input.type === 'number' ? Number(input.value) : input.value;
    }
    settings = await api('saveSettings', { settings: s });
    toast(t('saved'));
  });
  $('#quitBtn').onclick = run(async () => { await api('quit'); window.close(); });
  document.addEventListener('visibilitychange', () => { if (!document.hidden) refresh(); });
}

(async function main() {
  wire();
  try {
    const hello = await api('hello');
    platform = hello.platform;
    settings = hello.settings;
    lang = settings.lang === 'en' ? 'en' : 'fa';
    document.body.classList.add(platform);
    $('#version').textContent = t('version', hello.version);
  } catch (e) { toast(e.message, true); }
  applyLang();
  refresh();
  setInterval(refresh, 1500);
})();
