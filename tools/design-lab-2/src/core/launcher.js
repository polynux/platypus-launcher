// Shared launcher state for every design: one truth, five readings.
import { useCallback, useEffect, useMemo, useRef, useState } from 'react';
import { GAMES, RUNNERS, NATIVE, RECIPES, JOBS_SEED, NOW_FIXED, sessionsFor, art } from './data.js';

export const runnerOf = (id) => (id === 'native' ? NATIVE : RUNNERS.find((r) => r.id === id));
export const isInstalled = (g) => g.stores.some((s) => s.installed);
export const activeCopy = (g) => g.stores.find((s) => s.store === g.source) || g.stores[0];
export const artOf = (g) => {
  const a = g.appid ? art(g) : {};
  return {
    hero: g.heroOverride || a.hero,
    logo: g.logoOverride || a.logo,
    header: g.headerOverride || a.header,
    cover: g.coverOverride || a.cover,
  };
};

export function ago(ms) {
  const m = ms / 60e3;
  if (m < 1) return 'just now';
  if (m < 60) return `${Math.round(m)} min ago`;
  const h = m / 60;
  if (h < 24) return `${Math.round(h)} h ago`;
  const d = h / 24;
  if (d < 2) return 'yesterday';
  if (d < 30) return `${Math.round(d)} days ago`;
  if (d < 365) return `${Math.round(d / 30)} mo ago`;
  return `${Math.round(d / 365)} yr ago`;
}

export function agoShort(ms) {
  const m = ms / 60e3;
  if (m < 60) return `${Math.max(1, Math.round(m))}m`;
  const h = m / 60;
  if (h < 24) return `${Math.round(h)}h`;
  const d = h / 24;
  if (d < 365) return `${Math.round(d)}d`;
  return `${Math.round(d / 365)}y`;
}

export const clock = (t) =>
  new Date(t).toLocaleTimeString('en-GB', { hour: '2-digit', minute: '2-digit' });
export const dayLabel = (t) =>
  new Date(t).toLocaleDateString('en-GB', { weekday: 'short', day: '2-digit', month: 'short' });
export const dur = (min) => (min < 60 ? `${min} min` : `${Math.floor(min / 60)} h ${String(min % 60).padStart(2, '0')}`);

// The command Platypus would run. Each token carries the setting it comes from,
// so a design can light up exactly what the last edit changed.
export function launchTokens(g) {
  const t = [];
  const r = runnerOf(g.runner);
  if (!r) return t;
  if (g.gamemode) t.push({ key: 'gamemode', text: 'gamemoderun' });
  if (g.gamescope) t.push({ key: 'gamescope', text: `gamescope -W 2560 -H 1440 -r 165${g.fsr ? ' -F fsr' : ''} --` });
  if (g.mangohud) t.push({ key: 'mangohud', text: 'mangohud' });
  for (const [k, v] of g.env) t.push({ key: `env:${k}`, text: `${k}=${v}`, kind: 'env' });
  if (g.runner === 'native') {
    t.push({ key: 'runner', text: `./${g.id}.x86_64`, kind: 'bin' });
  } else {
    if (g.syncPrim && g.syncPrim !== 'NTSync') t.push({ key: 'syncPrim', text: g.syncPrim === 'Off' ? 'PROTON_NO_FSYNC=1 PROTON_NO_ESYNC=1' : `PROTON_NO_NTSYNC=1`, kind: 'env' });
    if (g.graphics && g.graphics.startsWith('WineD3D')) t.push({ key: 'graphics', text: 'PROTON_USE_WINED3D=1', kind: 'env' });
    if (g.recipes.length) t.push({ key: 'recipes', text: `umu-run --verbs ${g.recipes.join(',')}`, kind: 'wrap' });
    t.push({ key: 'runner', text: `${r.name}/proton run`, kind: 'bin' });
    t.push({ key: 'exe', text: `${g.title.replace(/[^A-Za-z0-9]/g, '')}.exe`, kind: 'bin' });
  }
  if (g.args) t.push({ key: 'args', text: g.args, kind: 'arg' });
  return t;
}

export const STATE_FILTERS = [
  { id: 'all', label: 'Everything' },
  { id: 'installed', label: 'Installed' },
  { id: 'not-installed', label: 'Not installed' },
  { id: 'attention', label: 'Needs attention' },
];
// Library views for sidebars that show them as places rather than filters.
export const VIEWS = [
  { id: 'all', label: 'All games' },
  { id: 'installed', label: 'Installed' },
  { id: 'downloads', label: 'Downloads' },
  { id: 'attention', label: 'Needs attention' },
];
export const TIER_ORDER = ['native', 'platinum', 'gold', 'silver', 'bronze', 'borked'];
export const sizeOf = (g) => parseFloat(activeCopy(g).size) || 0;
export const isDownloading = (g, jobs) => g.stores.some((s) => s.update) || jobs.some((j) => j.gameId === g.id && j.kind === 'download');
export function stateMatch(g, f) {
  if (f === 'installed') return isInstalled(g);
  if (f === 'not-installed') return !isInstalled(g);
  if (f === 'attention') return g.sync === 'conflict' || g.stores.some((s) => s.update) || g.tier === 'bronze' || g.tier === 'borked';
  return true;
}

function fuzzy(q, s) {
  if (!q) return 1;
  q = q.toLowerCase();
  s = s.toLowerCase();
  if (s.includes(q)) return 2 + (s.startsWith(q) ? 1 : 0);
  let i = 0;
  for (const c of s) if (c === q[i]) i++;
  return i === q.length ? 1 : 0;
}
export const matches = (q, s) => fuzzy(q, s) > 0;
export const score = fuzzy;

export function useLauncher({ initial = 'bg3', sort: initialSort = 'recent' } = {}) {
  const [sort, setSort] = useState(initialSort);
  const [games, setGames] = useState(() =>
    GAMES.map((g) => ({ ...g, source: g.stores.find((s) => s.installed)?.store || g.stores[0].store })),
  );
  const [selectedId, setSelectedId] = useState(initial);
  const [query, setQuery] = useState('');
  const [storeFilter, setStoreFilter] = useState('all');
  const [stores, setStores] = useState([]); // empty = every store
  const [stateFilter, setStateFilter] = useState('all');
  const [refine, setRefine] = useState(null); // optional extra filter from a design: { fn, key }
  const [inspector, setInspector] = useState(true);
  const [palette, setPalette] = useState(null); // null | { mode, gameId }
  const [conflictFor, setConflictFor] = useState(null);
  const [running, setRunning] = useState(null); // { id, startedAt, phase }
  const [jobs, setJobs] = useState(JOBS_SEED);
  const [log, setLog] = useState(() => [
    { t: NOW_FIXED - 9 * 60e3, src: 'core', text: 'library scan: 24 games across steam, gog, epic, local' },
    { t: NOW_FIXED - 7 * 60e3, src: 'steam', text: 'cloud: baldur’s gate 3 has a newer save from steamdeck' },
    { t: NOW_FIXED - 4 * 60e3, src: 'fossilize', text: 'nightreign: replaying 3,170 pipelines (4 threads)' },
    { t: NOW_FIXED - 2 * 60e3, src: 'epic', text: 'hades ii: upload queued, 1.2 MB' },
  ]);
  const [lastChange, setLastChange] = useState({ key: null, n: 0 });
  const [toast, setToast] = useState(null);
  const toastTimer = useRef();

  const say = useCallback((text, tone = 'ok') => {
    setToast({ text, tone, n: Date.now() });
    clearTimeout(toastTimer.current);
    toastTimer.current = setTimeout(() => setToast(null), 3200);
  }, []);
  const pushLog = useCallback((src, text) => setLog((l) => [...l.slice(-60), { t: Date.now(), src, text }]), []);

  const visible = useMemo(() => {
    const list = games.filter(
      (g) =>
        (storeFilter === 'all' || g.stores.some((s) => s.store === storeFilter)) &&
        (stores.length === 0 || g.stores.some((s) => stores.includes(s.store))) &&
        (stateFilter === 'downloads' ? isDownloading(g, jobs) : stateMatch(g, stateFilter)) &&
        matches(query, g.title) &&
        (!refine || refine.fn(g)),
    );
    const by = {
      recent: (a, b) => isInstalled(b) - isInstalled(a) || a.lastAgo - b.lastAgo,
      title: (a, b) => a.title.localeCompare(b.title),
      hours: (a, b) => b.hours - a.hours,
      last: (a, b) => a.lastAgo - b.lastAgo,
      year: (a, b) => b.year - a.year,
      size: (a, b) => sizeOf(b) - sizeOf(a),
      tier: (a, b) => TIER_ORDER.indexOf(a.tier) - TIER_ORDER.indexOf(b.tier),
    }[sort.replace(/-desc$/, '')];
    const out = [...list].sort(by);
    return sort.endsWith('-desc') ? out.reverse() : out;
  }, [games, query, storeFilter, sort, stores, stateFilter, refine?.key, jobs.map((j) => j.id).join()]);

  const game = games.find((g) => g.id === selectedId) || visible[0] || games[0];

  const patch = useCallback((id, fn, key) => {
    setGames((gs) => gs.map((g) => (g.id === id ? { ...g, ...fn(g) } : g)));
    if (key) setLastChange((c) => ({ key, n: c.n + 1 }));
  }, []);

  const move = useCallback(
    (d) => {
      const i = visible.findIndex((g) => g.id === game.id);
      const next = visible[Math.max(0, Math.min(visible.length - 1, (i < 0 ? 0 : i) + d))];
      if (next) setSelectedId(next.id);
    },
    [visible, game],
  );

  const actions = {
    select: setSelectedId,
    move,
    setQuery,
    setStoreFilter,
    setSort,
    toggleStore: (s) => setStores((cur) => (cur.includes(s) ? cur.filter((x) => x !== s) : [...cur, s])),
    setStores,
    setStateFilter,
    resetFilters: () => { setStores([]); setStateFilter('all'); setQuery(''); setStoreFilter('all'); },
    setRefine,
    toggleInspector: () => setInspector((v) => !v),
    setInspector,
    openPalette: (mode = 'root') => setPalette({ mode }),
    closePalette: () => setPalette(null),
    setPalette,
    openConflict: (id) => setConflictFor(id),
    closeConflict: () => setConflictFor(null),
    setRunner: (id, runner) => {
      patch(id, () => ({ runner }), 'runner');
      pushLog('core', `${id}: runner set to ${runnerOf(runner)?.name}`);
    },
    setOpt: (id, key, value) => patch(id, () => ({ [key]: value }), key),
    toggleOpt: (id, key) => patch(id, (g) => ({ [key]: !g[key] }), key),
    toggleRecipe: (id, rid) => {
      patch(id, (g) => ({ recipes: g.recipes.includes(rid) ? g.recipes.filter((r) => r !== rid) : [...g.recipes, rid] }), 'recipes');
    },
    setEnv: (id, i, k, v) =>
      patch(id, (g) => ({ env: g.env.map((e, j) => (j === i ? [k, v] : e)) }), `env:${k}`),
    addEnv: (id) => patch(id, (g) => ({ env: [...g.env, ['NEW_VARIABLE', '']] }), 'env:NEW_VARIABLE'),
    removeEnv: (id, i) => patch(id, (g) => ({ env: g.env.filter((_, j) => j !== i) }), 'env'),
    setSource: (id, store) => {
      patch(id, () => ({ source: store }));
      say(`Launching from ${store === 'gog' ? 'GOG' : store[0].toUpperCase() + store.slice(1)} from now on`);
    },
    play: (id) => {
      const g = games.find((x) => x.id === id);
      if (!g) return;
      if (!activeCopy(g).installed) {
        setJobs((j) => [...j.filter((x) => x.id !== `dl-${id}`), { id: `dl-${id}`, gameId: id, kind: 'download', label: `${g.title} · installing from ${activeCopy(g).store}`, progress: 0.01, rate: 0.01, unit: `0 / ${activeCopy(g).size}` }]);
        pushLog(activeCopy(g).store, `${g.title}: install queued (${activeCopy(g).size})`);
        say(`Installing ${g.title}`);
        return;
      }
      if (g.sync === 'conflict') {
        setConflictFor(id);
        return;
      }
      setRunning({ id, startedAt: Date.now(), phase: 'starting' });
      pushLog('core', `${g.title}: ${launchTokens(g).map((t) => t.text).join(' ')}`);
      setTimeout(() => setRunning((r) => (r && r.id === id ? { ...r, phase: 'running' } : r)), 1400);
    },
    stop: () => {
      if (running) pushLog('core', `${games.find((g) => g.id === running.id)?.title}: stopped, exit 0`);
      setRunning(null);
    },
    resolveConflict: (id, choice) => {
      patch(id, (g) => ({
        sync: 'synced',
        saves: g.saves.map((s) => (s.conflict ? { ...s, conflict: false, ...(choice === 'cloud' ? { local: s.cloud } : { cloud: s.local }) } : s)),
      }));
      const g = games.find((x) => x.id === id);
      const msg = choice === 'local' ? 'Kept this PC’s save, uploading' : choice === 'cloud' ? 'Downloaded the cloud save' : 'Kept both; the cloud copy is saved as a new slot';
      pushLog('sync', `${g?.title}: ${msg.toLowerCase()}`);
      say(msg);
      setConflictFor(null);
      setPalette(null);
    },
    verify: (id) => {
      const g = games.find((x) => x.id === id);
      setJobs((j) => [...j, { id: `verify-${id}-${Date.now()}`, gameId: id, kind: 'verify', label: `${g.title} · verifying files`, progress: 0.02, rate: 0.04, unit: activeCopy(g).size }]);
      say(`Verifying ${g.title}`);
    },
    say,
  };

  // Telemetry ticks.
  useEffect(() => {
    const t = setInterval(() => {
      setJobs((js) => {
        const next = [];
        for (const j of js) {
          const p = Math.min(1, j.progress + j.rate * (0.6 + Math.random() * 0.8));
          if (p >= 1) {
            setLog((l) => [...l.slice(-60), { t: Date.now(), src: j.kind, text: `${j.label.toLowerCase()}: done` }]);
            if (j.id === 'sync') setGames((gs) => gs.map((g) => (g.id === 'hades2' ? { ...g, sync: 'synced' } : g)));
            continue;
          }
          next.push({ ...j, progress: p });
        }
        return next;
      });
    }, 700);
    return () => clearInterval(t);
  }, []);

  // Global keys.
  useEffect(() => {
    const onKey = (e) => {
      const typing = /INPUT|TEXTAREA|SELECT/.test(document.activeElement?.tagName) || document.activeElement?.isContentEditable;
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'k') {
        e.preventDefault();
        setPalette((p) => (p ? null : { mode: 'root' }));
        return;
      }
      if ((e.ctrlKey || e.metaKey) && e.key.toLowerCase() === 'i') {
        e.preventDefault();
        setInspector((v) => !v);
        return;
      }
      if (palette || conflictFor || typing) return;
      if (e.target.closest?.('[role="listbox"],[role="menu"],[role="dialog"],.fm-panel,[data-keys-local]')) return;
      if (e.key === 'ArrowDown' || e.key === 'j') { e.preventDefault(); move(1); }
      else if (e.key === 'ArrowUp' || e.key === 'k') { e.preventDefault(); move(-1); }
      else if (e.key === '/') { e.preventDefault(); document.querySelector('[data-search]')?.focus(); }
      else if (e.key === 'Enter' && e.target === document.body) { actions.play(game.id); }
    };
    window.addEventListener('keydown', onKey);
    return () => window.removeEventListener('keydown', onKey);
  });

  // Keep the selected spine row in view.
  useEffect(() => {
    document.querySelector(`[data-game-row="${game.id}"]`)?.scrollIntoView({ block: 'nearest' });
  }, [game.id]);

  const now = Date.now();
  const sessions = useMemo(() => sessionsFor(game, NOW_FIXED), [game.id]);

  return {
    games, visible, game, query, storeFilter, sort, stores, stateFilter, inspector, palette, conflictFor, running, jobs, log,
    lastChange, toast, sessions, now, actions,
    tokens: launchTokens(game),
    recipes: RECIPES,
    runners: RUNNERS,
  };
}

// Average the saturated pixels of a key art image (Steam's CDN serves CORS).
const toneCache = new Map();
export function useArtTone(url) {
  const [tone, setTone] = useState(() => toneCache.get(url) || null);
  useEffect(() => {
    if (!url) return;
    if (toneCache.has(url)) { setTone(toneCache.get(url)); return; }
    let live = true;
    const img = new Image();
    img.crossOrigin = 'anonymous';
    img.onload = () => {
      try {
        const c = document.createElement('canvas');
        c.width = 48; c.height = 16;
        const x = c.getContext('2d', { willReadFrequently: true });
        x.drawImage(img, 0, 0, 48, 16);
        const d = x.getImageData(0, 0, 48, 16).data;
        let r = 0, g = 0, b = 0, w = 0;
        for (let i = 0; i < d.length; i += 4) {
          const mx = Math.max(d[i], d[i + 1], d[i + 2]);
          const mn = Math.min(d[i], d[i + 1], d[i + 2]);
          const sat = mx ? (mx - mn) / mx : 0;
          const wt = sat * sat * (mx / 255) + 0.002;
          r += d[i] * wt; g += d[i + 1] * wt; b += d[i + 2] * wt; w += wt;
        }
        const t = [r / w, g / w, b / w].map(Math.round);
        toneCache.set(url, t);
        if (live) setTone(t);
      } catch { /* tainted canvas: keep neutral */ }
    };
    img.src = url;
    return () => { live = false; };
  }, [url]);
  return tone;
}

export function useTick(ms = 1000) {
  const [, set] = useState(0);
  useEffect(() => {
    const t = setInterval(() => set((n) => n + 1), ms);
    return () => clearInterval(t);
  }, [ms]);
}

// Push a sampled tone into a usable signal colour (vivid, mid-light).
export function vivid(rgb, l = 0.62, sMin = 0.55) {
  if (!rgb) return null;
  let [r, g, b] = rgb.map((v) => v / 255);
  const mx = Math.max(r, g, b), mn = Math.min(r, g, b);
  let h = 0, s = 0;
  const d = mx - mn;
  if (d) {
    s = d / (1 - Math.abs(mx + mn - 1));
    h = mx === r ? ((g - b) / d) % 6 : mx === g ? (b - r) / d + 2 : (r - g) / d + 4;
    h *= 60;
    if (h < 0) h += 360;
  }
  s = Math.max(s, sMin);
  const c = (1 - Math.abs(2 * l - 1)) * s;
  const x = c * (1 - Math.abs(((h / 60) % 2) - 1));
  const m = l - c / 2;
  const [a, bb, cc] = h < 60 ? [c, x, 0] : h < 120 ? [x, c, 0] : h < 180 ? [0, c, x] : h < 240 ? [0, x, c] : h < 300 ? [x, 0, c] : [c, 0, x];
  return `rgb(${Math.round((a + m) * 255)} ${Math.round((bb + m) * 255)} ${Math.round((cc + m) * 255)})`;
}

export function hslOf(rgb) {
  if (!rgb) return null;
  const [r, g, b] = rgb.map((v) => v / 255);
  const mx = Math.max(r, g, b), mn = Math.min(r, g, b);
  const l = (mx + mn) / 2;
  const d = mx - mn;
  if (!d) return [0, 0, l];
  const s = d / (1 - Math.abs(2 * l - 1));
  let h = mx === r ? ((g - b) / d) % 6 : mx === g ? (b - r) / d + 2 : (r - g) / d + 4;
  h *= 60;
  if (h < 0) h += 360;
  return [Math.round(h), s, l];
}
// hsl() string from a sampled tone at a chosen saturation/lightness.
export const toneAt = (rgb, s, l, fallback) => {
  const t = hslOf(rgb);
  if (!t) return fallback;
  return `hsl(${t[0]} ${Math.round(Math.min(Math.max(t[1], 0.6), 1) * s * 100)}% ${Math.round(l * 100)}%)`;
};
