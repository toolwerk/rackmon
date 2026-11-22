(() => {
  'use strict';

  // Bildschirm 1424 x 280, Kacheln 340 x 246, bei vier Spalten 356 Punkte Abstand.
  const W = 1424, H = 280, TW = 340, TH = 246, MARGIN = 8, GAP = 16, HIST = 60;
  const C = {
    bg: '#0e1014', tile: '#171a21', line: '#262b35', txt: '#e6e8ec', mut: '#8b919c',
    cpu: '#4c9aff', ram: '#2fc4b2', disk: '#9a8cf0', ok: '#3fb982', warn: '#e8a73a', bad: '#e5534b',
    temp: '#f08a4b',
  };

  const params = new URLSearchParams(location.search);
  const screenId = parseInt(location.pathname.split('/').pop(), 10) || 1;
  const pinned = params.has('page') ? Math.max(0, parseInt(params.get('page'), 10) - 1 || 0) : null;

  let state = null, page = 0, connected = false, lastMsg = 0, first = true;

  const svg = document.getElementById('screen');
  svg.innerHTML = '<rect width="' + W + '" height="' + H + '" fill="' + C.bg + '"/><g id="pg"></g><g id="foot"></g>';
  const pg = document.getElementById('pg');
  const foot = document.getElementById('foot');

  const esc = (s) => String(s).replace(/[&<>"']/g, (c) => ({ '&': '&amp;', '<': '&lt;', '>': '&gt;', '"': '&quot;', "'": '&#39;' }[c]));
  const clamp = (v, a, b) => Math.max(a, Math.min(b, v));
  const num = (v, d) => v.toLocaleString('de-DE', { minimumFractionDigits: d, maximumFractionDigits: d });

  function rate(b) {
    if (b == null) return '–';
    if (b < 1000) return Math.round(b) + ' B/s';
    if (b < 1e6) return Math.round(b / 1024) + ' KB/s';
    return num(b / 1048576, 1) + ' MB/s';
  }
  function gb(b) { return num(b / 1073741824, 1) + ' GB'; }
  function uptime(s) {
    if (!s) return '–';
    if (s < 3600) return Math.max(1, Math.round(s / 60)) + ' Min';
    if (s < 86400) return Math.round(s / 3600) + ' Std';
    const d = Math.floor(s / 86400);
    return d + (d === 1 ? ' Tag' : ' Tage');
  }
  function hhmm(ts) {
    const d = new Date(ts * 1000);
    return String(d.getHours()).padStart(2, '0') + ':' + String(d.getMinutes()).padStart(2, '0');
  }

  // Farbe nach Prozentwert: normal, gelb, rot.
  function sev(v, th, normal) { return v >= th.crit ? C.bad : v >= th.warn ? C.warn : normal; }
  function tempColor(v, th) { return v >= th.temp_crit ? C.bad : v >= th.temp_warn ? C.warn : C.temp; }

  function spark(vals, x, y, w, h, lo, hi, col, thr, th) {
    if (!vals || vals.length < 2) return '';
    const step = w / (HIST - 1), n = vals.length;
    const pts = vals.map((v, i) => [x + w - (n - 1 - i) * step, y + h - clamp((v - lo) / (hi - lo), 0, 1) * h]);
    const line = 'M' + pts.map((p) => p[0].toFixed(1) + ',' + p[1].toFixed(1)).join('L');
    const e = pts[n - 1], s0 = pts[0];
    let out = '<path d="' + line + 'L' + e[0].toFixed(1) + ',' + (y + h) + 'L' + s0[0].toFixed(1) + ',' + (y + h) + 'Z" fill="' + col + '" opacity=".14"/>' +
      '<path d="' + line + '" fill="none" stroke="' + col + '" stroke-width="2" stroke-linejoin="round" stroke-linecap="round"/>';
    if (thr != null) {
      const ty = y + h - clamp((thr - lo) / (hi - lo), 0, 1) * h;
      out += '<line x1="' + x + '" x2="' + (x + w) + '" y1="' + ty.toFixed(1) + '" y2="' + ty.toFixed(1) + '" stroke="' + C.mut + '" stroke-dasharray="3 4" opacity=".6"/>';
    }
    return out + '<circle cx="' + e[0].toFixed(1) + '" cy="' + e[1].toFixed(1) + '" r="4" fill="' + col + '" stroke="' + C.tile + '" stroke-width="2"/>';
  }

  function bar(y, label, v, text, col) {
    const w = 188 * clamp(v, 0, 100) / 100;
    return '<text x="14" y="' + (y + 9) + '" font-size="12" fill="' + C.mut + '">' + label + '</text>' +
      '<rect x="58" y="' + y + '" width="188" height="8" rx="4" fill="' + C.line + '"/>' +
      '<rect x="58" y="' + y + '" width="' + w.toFixed(1) + '" height="8" rx="4" fill="' + col + '"/>' +
      '<text x="260" y="' + (y + 9) + '" font-size="12" fill="' + C.txt + '">' + text + '</text>';
  }

  function tile(v, x, y, scale, th) {
    const s = v.sample, off = v.state === 'offline', stale = v.state === 'stale';
    const dot = off ? C.bad : stale ? C.warn : C.ok;
    const opacity = off ? 0.55 : stale ? 0.8 : 1;
    let o = '<g transform="translate(' + x.toFixed(1) + ',' + y.toFixed(1) + ') scale(' + scale.toFixed(4) + ')" opacity="' + opacity + '">' +
      '<rect width="' + TW + '" height="' + TH + '" rx="10" fill="' + C.tile + '" stroke="' + (off ? '#5a2a2a' : C.line) + '"/>';
    const right = off ? (v.last_seen ? 'offline seit ' + hhmm(v.last_seen) : 'noch nie gemeldet') : v.role;
    o += '<circle cx="20" cy="20" r="5" fill="' + dot + '"/>' +
      '<text x="32" y="25" font-size="17" font-weight="500" fill="' + C.txt + '">' + esc(v.name) + '</text>' +
      '<text x="326" y="25" font-size="12" fill="' + (off ? C.bad : C.mut) + '" text-anchor="end">' + esc(right) + '</text>';
    if (!s) return o + '</g>';

    // Prozessor: Ring und Verlauf
    const cc = sev(s.cpu, th, C.cpu), r = 26, k = 2 * Math.PI * r;
    o += '<circle cx="42" cy="66" r="' + r + '" fill="none" stroke="' + C.line + '" stroke-width="7"/>' +
      '<circle cx="42" cy="66" r="' + r + '" fill="none" stroke="' + cc + '" stroke-width="7" stroke-linecap="round" stroke-dasharray="' +
      (k * clamp(s.cpu, 0, 100) / 100).toFixed(1) + ' ' + k.toFixed(1) + '" transform="rotate(-90 42 66)"/>' +
      '<text x="42" y="72" font-size="17" font-weight="500" fill="' + C.txt + '" text-anchor="middle">' + Math.round(s.cpu) + '%</text>' +
      '<text x="92" y="42" font-size="12" fill="' + C.mut + '">Prozessor, 5 Minuten</text>' +
      spark(v.cpu_hist, 92, 48, 236, 46, 0, 100, cc);

    // Temperatur
    if (s.temp != null) {
      const tc = tempColor(s.temp, th), hist = v.temp_hist || [];
      const lo = Math.min(30, ...hist, s.temp) - 2, hi = Math.max(90, ...hist, s.temp) + 2;
      o += '<text x="14" y="128" font-size="26" font-weight="500" fill="' + C.txt + '">' + Math.round(s.temp) + '°</text>' +
        '<text x="14" y="146" font-size="12" fill="' + C.mut + '">Temperatur</text>' +
        '<text x="92" y="118" font-size="12" fill="' + C.mut + '">Grenze ' + Math.round(th.temp_warn) + '°</text>' +
        spark(hist, 92, 122, 236, 38, lo, hi, tc, th.temp_warn);
      if (s.power_warn) {
        o += '<path d="M62 110l-8 13h6l-3 11 9-14h-6z" fill="' + C.warn + '"/>';
      }
    } else {
      o += '<text x="14" y="128" font-size="26" font-weight="500" fill="' + C.mut + '">–</text>' +
        '<text x="14" y="146" font-size="12" fill="' + C.mut + '">Temperatur</text>' +
        '<text x="92" y="146" font-size="12" fill="' + C.mut + '">kein Sensor</text>';
    }

    // Speicher und Datenträger
    o += bar(172, 'RAM', s.mem, Math.round(s.mem) + ' %  ' + gb(s.mem_used), sev(s.mem, th, C.ram)) +
      bar(190, 'Disk', s.disk, Math.round(s.disk) + ' %', sev(s.disk, th, C.disk));

    // Fuß: Netz, Jobs, Laufzeit
    o += '<text x="14" y="232" font-size="12" fill="' + C.txt + '">↓ ' + rate(s.net_rx) + '</text>' +
      '<text x="104" y="232" font-size="12" fill="' + C.txt + '">↑ ' + rate(s.net_tx) + '</text>';
    if (s.ci_jobs > 0) {
      const label = s.ci_jobs + (s.ci_jobs === 1 ? ' Job' : ' Jobs');
      o += '<rect x="196" y="219" width="64" height="18" rx="9" fill="' + C.cpu + '" opacity=".2"/>' +
        '<text x="228" y="232" font-size="12" fill="' + C.cpu + '" text-anchor="middle">' + label + '</text>';
    }
    return o + '<text x="326" y="232" font-size="12" fill="' + C.mut + '" text-anchor="end">' + uptime(s.uptime) + '</text></g>';
  }

  function screenCfg() {
    const list = (state && state.screens) || [];
    return list.find((s) => s.id === screenId) || list[0] || { id: 1, hosts: [], per_page: 4, page_seconds: 8 };
  }

  function render() {
    if (!state) {
      pg.innerHTML = '<text x="712" y="150" font-size="18" fill="' + C.mut + '" text-anchor="middle">Warte auf den Server …</text>';
      foot.innerHTML = '';
      return;
    }
    const sc = screenCfg(), th = state.thresholds;
    const byId = new Map(state.hosts.map((h) => [h.id, h]));
    const views = sc.hosts.map((id) => byId.get(id)).filter(Boolean);
    const pp = sc.per_page, pages = Math.max(1, Math.ceil(views.length / pp));
    const tw = (W - 2 * MARGIN - (pp - 1) * GAP) / pp, scale = tw / TW, ty = 6 + (TH - TH * scale) / 2;
    if (pinned != null) page = Math.min(pinned, pages - 1);
    if (page >= pages) page = 0;

    let o = '';
    views.forEach((v, i) => {
      const p = Math.floor(i / pp), col = i % pp;
      o += tile(v, p * W + MARGIN + col * (tw + GAP), ty, scale, th);
    });
    pg.innerHTML = o;
    if (first) pg.style.transition = 'none';
    pg.style.transform = 'translateX(' + (-page * W) + 'px)';
    if (first) { void pg.getBoundingClientRect(); pg.style.transition = ''; first = false; }

    const online = views.filter((v) => v.state === 'ok').length;
    let f = '<text x="16" y="274" font-size="11" fill="' + C.mut + '">' + hhmm(Date.now() / 1000) + '</text>';
    if (!connected) f += '<text x="1408" y="274" font-size="11" fill="' + C.bad + '" text-anchor="end">Server nicht erreichbar</text>';
    else f += '<text x="1408" y="274" font-size="11" fill="' + C.mut + '" text-anchor="end">' + online + ' von ' + views.length + ' online</text>';
    if (pages > 1) {
      const x0 = W / 2 - (pages - 1) * 8;
      for (let i = 0; i < pages; i++) {
        f += '<circle cx="' + (x0 + i * 16) + '" cy="270" r="3.5" fill="' + (i === page ? C.txt : C.line) + '"/>';
      }
    }
    foot.innerHTML = f;
  }

  // Blättern
  function turn() {
    if (!state || pinned != null) return;
    const sc = screenCfg(), pages = Math.max(1, Math.ceil(sc.hosts.length / sc.per_page));
    if (pages > 1) { page = (page + 1) % pages; render(); }
  }
  let turnTimer = null, turnEvery = 0;
  function schedule() {
    const every = screenCfg().page_seconds * 1000;
    if (every !== turnEvery) {
      clearInterval(turnTimer);
      turnEvery = every;
      turnTimer = setInterval(turn, every);
    }
  }

  // Verbindung
  const es = new EventSource('/events');
  es.onmessage = (e) => {
    try { state = JSON.parse(e.data); } catch (_) { return; }
    connected = true; lastMsg = Date.now();
    schedule(); render();
  };
  es.onerror = () => { connected = false; render(); };
  setInterval(() => {
    if (connected && Date.now() - lastMsg > 5000) { connected = false; render(); }
  }, 1000);
  render();
})();
