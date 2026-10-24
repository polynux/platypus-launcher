// The game's store page: the real description (text, pictures, looping clips),
// screenshots with thumbnails, achievements and facts. Content is a snapshot
// of each game's Steam page (scripts/fetch-store-pages.mjs); unlock state is
// made up from play time so every game tells a believable story.
import React, { useEffect, useMemo, useRef, useState } from 'react';
import { ChevronLeft, ChevronRight, X, Lock, Trophy, Maximize2, EyeOff } from 'lucide-react';
import PAGES from './storePages.json';
import { ago } from './launcher.js';

export const pageOf = (g) => PAGES[g.id] || null;

const iconUrl = (appid, file) => `https://shared.fastly.steamstatic.com/community_assets/images/apps/${appid}/${file}`;

function rng(seed) {
  let h = 2166136261;
  for (const c of seed) h = Math.imul(h ^ c.charCodeAt(0), 16777619);
  return () => {
    h = Math.imul(h ^ (h >>> 15), 2246822507);
    h = Math.imul(h ^ (h >>> 13), 3266489909);
    return ((h ^= h >>> 16) >>> 0) / 4294967296;
  };
}

const D = 864e5;
const achCache = new Map();
// Who unlocked what: common achievements first, rarer ones later, spread over the play time.
export function achievementsOf(g) {
  if (achCache.has(g.id)) return achCache.get(g.id);
  const p = pageOf(g);
  if (!p || !p.ach.list.length) { achCache.set(g.id, null); return null; }
  const r = rng(g.id);
  const f = g.hours <= 0 ? 0 : Math.min(0.94, 0.08 + (g.hours / (g.hours + 55)) * 0.88);
  const ranked = p.ach.list
    .map((a) => ({ a, s: (a.pct ?? 40) + r() * 30 }))
    .sort((x, y) => y.s - x.s);
  const n = Math.round(f * ranked.length);
  const span = Math.min(420 * D, Math.max(4 * D, g.hours * 1.6 * D));
  const got = new Map();
  ranked.slice(0, n).forEach(({ a }, i) => {
    const t = n > 1 ? 1 - i / (n - 1) : 0;
    got.set(a.id, g.lastAgo + span * t * (0.85 + r() * 0.15) + r() * 4 * 3600e3);
  });
  const list = p.ach.list.map((a) => ({
    ...a,
    icon: iconUrl(p.appid, a.icon),
    gray: iconUrl(p.appid, a.gray),
    got: got.has(a.id),
    at: got.get(a.id) ?? null,
  }));
  const unlockedListed = list.filter((a) => a.got);
  const out = {
    total: p.ach.total,
    unlocked: Math.round((unlockedListed.length / list.length) * p.ach.total),
    list,
    recent: [...unlockedListed].sort((x, y) => x.at - y.at).slice(0, 4),
    rarest: [...unlockedListed].sort((x, y) => (x.pct ?? 100) - (y.pct ?? 100))[0] || null,
    next: list.filter((a) => !a.got && !a.hidden).sort((x, y) => (y.pct ?? 0) - (x.pct ?? 0)).slice(0, 3),
  };
  achCache.set(g.id, out);
  return out;
}

export const rarityLabel = (pct) => (pct == null ? null : pct < 5 ? 'Ultra rare' : pct < 15 ? 'Rare' : pct < 40 ? 'Uncommon' : 'Common');
const pctText = (pct) => (pct == null ? '' : `${pct < 1 ? pct.toFixed(1) : Math.round(pct)}% of players`);

const reduced = () => window.matchMedia('(prefers-reduced-motion: reduce)').matches;

// Steam pages loop short clips like GIFs. Play only while on screen, never with reduced motion.
function Clip({ src, poster }) {
  const ref = useRef();
  useEffect(() => {
    const v = ref.current;
    if (!v || reduced()) return;
    const io = new IntersectionObserver(([e]) => (e.isIntersecting ? v.play().catch(() => {}) : v.pause()), { threshold: 0.2 });
    io.observe(v);
    return () => io.disconnect();
  }, [src]);
  return <video ref={ref} className="pg-media" src={src} poster={poster} muted loop playsInline preload="none" aria-hidden />;
}

function Pic({ src }) {
  const [ok, setOk] = useState(true);
  if (!ok) return null;
  return <img className="pg-media" src={src} alt="" loading="lazy" decoding="async" onError={() => setOk(false)} />;
}

// The full description. `clamp` folds it to a height with a "Read more".
export function StoreAbout({ g, clamp, className = '' }) {
  const p = pageOf(g);
  const [open, setOpen] = useState(false);
  useEffect(() => setOpen(false), [g.id]);
  if (!p) return <p className="pg-empty">This game has no store page. Platypus found it on disk.</p>;
  const folded = clamp && !open;
  return (
    <div className={`pg-about ${folded ? 'is-folded' : ''} ${className}`} style={folded ? { '--pg-clamp': `${clamp}px` } : undefined}>
      <div className="pg-about-body">
        {p.about.map((b, i) => {
          if (b.t === 'h') return <h3 key={i} className="pg-h">{b.text}</h3>;
          if (b.t === 'ul') return <ul key={i} className="pg-ul">{b.items.map((x, j) => <li key={j}>{x}</li>)}</ul>;
          if (b.t === 'img') return <figure key={i} className="pg-fig"><Pic src={b.src} /></figure>;
          if (b.t === 'video') return <figure key={i} className="pg-fig"><Clip src={b.src} poster={b.poster} /></figure>;
          return <p key={i} className="pg-p">{b.text}</p>;
        })}
      </div>
      {clamp && (
        <button type="button" className="pg-more" aria-expanded={open} onClick={() => setOpen(!open)}>{open ? 'Show less' : 'Read the full description'}</button>
      )}
    </div>
  );
}

// Full-screen viewer for screenshots. The one place a blur is allowed: it is transient.
export function Lightbox({ shots, index, onIndex, onClose, title }) {
  useEffect(() => {
    const prev = document.activeElement;
    const key = (e) => {
      if (e.key === 'Escape') { e.stopPropagation(); onClose(); }
      if (e.key === 'ArrowRight') onIndex((index + 1) % shots.length);
      if (e.key === 'ArrowLeft') onIndex((index + shots.length - 1) % shots.length);
    };
    window.addEventListener('keydown', key, true);
    return () => { window.removeEventListener('keydown', key, true); prev?.focus?.(); };
  }, [index, shots.length]);
  const ref = useRef();
  useEffect(() => { ref.current?.focus(); }, []);
  return (
    <div className="pg-lightbox" role="dialog" aria-modal="true" aria-label={`${title} screenshots`} onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
      <button type="button" ref={ref} className="pg-lb-x" aria-label="Close" onClick={onClose}><X size={20} /></button>
      <button type="button" className="pg-lb-nav is-prev" aria-label="Previous screenshot" onClick={() => onIndex((index + shots.length - 1) % shots.length)}><ChevronLeft size={26} /></button>
      <img key={index} className="pg-lb-img" src={shots[index].full} alt={`${title}, screenshot ${index + 1} of ${shots.length}`} />
      <button type="button" className="pg-lb-nav is-next" aria-label="Next screenshot" onClick={() => onIndex((index + 1) % shots.length)}><ChevronRight size={26} /></button>
      <div className="pg-lb-strip">
        {shots.map((s, i) => (
          <button key={i} type="button" className={`pg-lb-thumb ${i === index ? 'is-on' : ''}`} aria-label={`Screenshot ${i + 1}`} aria-current={i === index} onClick={() => onIndex(i)}>
            <img src={s.thumb} alt="" loading="lazy" />
          </button>
        ))}
      </div>
      <div className="pg-lb-count" aria-live="polite">{index + 1} / {shots.length}</div>
    </div>
  );
}

// Screenshots. "viewer": a large shot over a thumbnail strip. "sheet": thumbnails only.
export function Gallery({ g, variant = 'viewer', max, className = '' }) {
  const p = pageOf(g);
  const [i, setI] = useState(0);
  const [lb, setLb] = useState(null);
  const strip = useRef();
  useEffect(() => { setI(0); setLb(null); }, [g.id]);
  useEffect(() => {
    strip.current?.querySelector('.is-on')?.scrollIntoView({ block: 'nearest', inline: 'nearest' });
  }, [i]);
  if (!p?.shots.length) return null;
  const shots = max ? p.shots.slice(0, max) : p.shots;
  const onKey = (e) => {
    if (e.key === 'ArrowRight' || e.key === 'ArrowLeft') {
      e.preventDefault();
      e.stopPropagation();
      const n = (i + (e.key === 'ArrowRight' ? 1 : shots.length - 1)) % shots.length;
      setI(n);
      strip.current?.querySelectorAll('button')[n]?.focus();
    }
  };
  const thumbs = (
    <div className="pg-thumbs" ref={strip} role="group" aria-label="Screenshots" onKeyDown={onKey}>
      {shots.map((s, n) => (
        <button key={n} type="button" className={`pg-thumb ${variant === 'viewer' && n === i ? 'is-on' : ''}`} aria-label={`Screenshot ${n + 1} of ${shots.length}`} aria-current={variant === 'viewer' && n === i} onClick={() => (variant === 'viewer' ? setI(n) : setLb(n))}>
          <img src={s.thumb} alt="" loading="lazy" decoding="async" />
        </button>
      ))}
    </div>
  );
  return (
    <div className={`pg-gallery is-${variant} ${className}`}>
      {variant === 'viewer' && (
        <div className="pg-stage">
          <button type="button" className="pg-stage-btn" aria-label="View full screen" onClick={() => setLb(i)}>
            <img key={shots[i].thumb} className="pg-stage-img" src={shots[i].full} alt={`${g.title}, screenshot ${i + 1}`} />
            <span className="pg-stage-zoom" aria-hidden><Maximize2 size={15} /></span>
          </button>
          <button type="button" className="pg-stage-nav is-prev" aria-label="Previous screenshot" onClick={() => setI((i + shots.length - 1) % shots.length)}><ChevronLeft size={20} /></button>
          <button type="button" className="pg-stage-nav is-next" aria-label="Next screenshot" onClick={() => setI((i + 1) % shots.length)}><ChevronRight size={20} /></button>
        </div>
      )}
      {thumbs}
      {lb != null && <Lightbox shots={shots} index={lb} onIndex={setLb} onClose={() => setLb(null)} title={g.title} />}
    </div>
  );
}

export function AchIcon({ a, size = 40 }) {
  return (
    <span className={`pg-ach-icon ${a.got ? '' : 'is-locked'}`} style={{ width: size, height: size }}>
      <img src={a.got ? a.icon : a.gray} alt="" loading="lazy" />
      {!a.got && <Lock size={Math.round(size * 0.32)} className="pg-ach-lock" aria-hidden />}
    </span>
  );
}

// Compact: the count, a bar, what you unlocked last and what is closest.
export function AchSummary({ g, onOpen, showNext = true }) {
  const A = achievementsOf(g);
  if (!A) return <p className="pg-empty">No achievements for this game.</p>;
  const pct = A.total ? A.unlocked / A.total : 0;
  return (
    <div className="pg-achsum">
      <div className="pg-achsum-head">
        <Trophy size={16} aria-hidden />
        <span className="pg-achsum-count"><b>{A.unlocked}</b> of {A.total}</span>
        <span className="pg-achsum-pct">{Math.round(pct * 100)}%</span>
      </div>
      <div className="pg-bar" role="progressbar" aria-label="Achievements unlocked" aria-valuemin={0} aria-valuemax={A.total} aria-valuenow={A.unlocked}><span style={{ width: `${pct * 100}%` }} /></div>
      {A.recent.length > 0 && (
        <ul className="pg-achsum-list">
          {A.recent.slice(0, 3).map((a) => (
            <li key={a.id}><AchIcon a={a} size={34} /><span className="pg-achsum-text"><span>{a.name}</span><small>{ago(a.at)}{a.pct != null && a.pct < 15 ? `, ${rarityLabel(a.pct).toLowerCase()}` : ''}</small></span></li>
          ))}
        </ul>
      )}
      {showNext && A.next.length > 0 && A.unlocked < A.total && (
        <div className="pg-achsum-next">
          <span className="pg-achsum-sub">Up next</span>
          <div className="pg-achsum-icons">{A.next.map((a) => <span key={a.id} title={`${a.name}: ${a.desc}`}><AchIcon a={a} size={30} /></span>)}</div>
        </div>
      )}
      {onOpen && <button type="button" className="pg-link" onClick={onOpen}>See all {A.total} achievements</button>}
    </div>
  );
}

// Everything, with filters. Hidden ones keep their secret until unlocked.
export function AchList({ g, className = '' }) {
  const A = achievementsOf(g);
  const [show, setShow] = useState('all');
  const [sort, setSort] = useState('recent');
  useEffect(() => { setShow('all'); }, [g.id]);
  const items = useMemo(() => {
    if (!A) return [];
    let l = A.list.filter((a) => (show === 'all' ? true : show === 'got' ? a.got : !a.got));
    if (sort === 'rare') l = [...l].sort((x, y) => (x.pct ?? 100) - (y.pct ?? 100));
    else l = [...l].sort((x, y) => (x.got === y.got ? (x.got ? x.at - y.at : (y.pct ?? 0) - (x.pct ?? 0)) : x.got ? -1 : 1));
    return l;
  }, [A, show, sort]);
  if (!A) return <p className="pg-empty">No achievements for this game.</p>;
  const nGot = A.list.filter((a) => a.got).length;
  return (
    <div className={`pg-achlist ${className}`}>
      <div className="pg-achlist-tools">
        <div className="pg-seg" role="radiogroup" aria-label="Show">
          {[['all', `All ${A.list.length}`], ['got', `Unlocked ${nGot}`], ['locked', `Locked ${A.list.length - nGot}`]].map(([id, label]) => (
            <button key={id} type="button" role="radio" aria-checked={show === id} className={show === id ? 'is-on' : ''} onClick={() => setShow(id)}>{label}</button>
          ))}
        </div>
        <div className="pg-seg" role="radiogroup" aria-label="Sort">
          {[['recent', 'Recent'], ['rare', 'Rarest']].map(([id, label]) => (
            <button key={id} type="button" role="radio" aria-checked={sort === id} className={sort === id ? 'is-on' : ''} onClick={() => setSort(id)}>{label}</button>
          ))}
        </div>
      </div>
      <ul className="pg-achrows">
        {items.map((a) => {
          const secret = a.hidden && !a.got;
          return (
            <li key={a.id} className={`pg-achrow ${a.got ? 'is-got' : ''}`}>
              <AchIcon a={a} size={44} />
              <span className="pg-achrow-text">
                <span className="pg-achrow-name">{secret ? 'Hidden achievement' : a.name}</span>
                <span className="pg-achrow-desc">{secret ? <><EyeOff size={12} aria-hidden /> Keep playing to reveal it.</> : a.desc}</span>
              </span>
              <span className="pg-achrow-meta">
                {a.got ? <span className="pg-achrow-when">Unlocked {ago(a.at)}</span> : <span className="pg-achrow-when is-locked">Locked</span>}
                {a.pct != null && <span className={`pg-rar pg-rar-${rarityLabel(a.pct).split(' ').join('-').toLowerCase()}`}>{pctText(a.pct)}</span>}
              </span>
            </li>
          );
        })}
      </ul>
      {A.total > A.list.length && <p className="pg-note">Showing {A.list.length} of {A.total}.</p>}
    </div>
  );
}

export function GameFacts({ g, className = '' }) {
  const p = pageOf(g);
  if (!p) return null;
  return (
    <dl className={`pg-facts ${className}`}>
      <div><dt>Developer</dt><dd>{g.dev}</dd></div>
      {p.publisher && p.publisher !== g.dev && <div><dt>Publisher</dt><dd>{p.publisher}</dd></div>}
      {p.released && <div><dt>Released</dt><dd>{p.released}</dd></div>}
      {p.genres.length > 0 && <div><dt>Genre</dt><dd>{p.genres.slice(0, 3).join(', ')}</dd></div>}
      {p.metacritic && <div><dt>Metacritic</dt><dd>{p.metacritic}</dd></div>}
      {p.features.length > 0 && <div className="pg-facts-wide"><dt>Features</dt><dd className="pg-tags">{[...new Set(p.features.filter((f) => !/Steam|Family Sharing|Remote Play|DualShock|DualSense|Controller Support$/.test(f)))].slice(0, 5).map((f) => <span key={f}>{f}</span>)}</dd></div>}
    </dl>
  );
}

// A modal page for long lists (all achievements).
export function PageModal({ title, sub, onClose, children, className = '' }) {
  const ref = useRef();
  useEffect(() => {
    const prev = document.activeElement;
    ref.current?.querySelector('button')?.focus();
    const key = (e) => { if (e.key === 'Escape') { e.stopPropagation(); onClose(); } };
    window.addEventListener('keydown', key, true);
    return () => { window.removeEventListener('keydown', key, true); prev?.focus?.(); };
  }, []);
  return (
    <div className="scrim pg-scrim" onPointerDown={(e) => e.target === e.currentTarget && onClose()}>
      <div ref={ref} className={`pg-modal ${className}`} role="dialog" aria-modal="true" aria-label={title}>
        <header className="pg-modal-head">
          <div><h2>{title}</h2>{sub && <span>{sub}</span>}</div>
          <button type="button" className="icon-btn" aria-label="Close" onClick={onClose}><X size={16} /></button>
        </header>
        <div className="pg-modal-body">{children}</div>
      </div>
    </div>
  );
}
