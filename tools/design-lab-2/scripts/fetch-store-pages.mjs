// Snapshots real store pages (description, screenshots, achievements) from
// Steam into src/core/storePages.json so the lab renders real content offline.
// Run: bun scripts/fetch-store-pages.mjs
import { writeFileSync } from 'node:fs';
import { GAMES } from '../src/core/data.js';

const EXTRA = { teardown: 1167630 }; // local install, metadata from its Steam page
const MAX_ACH = 64;
const MAX_SHOTS = 10;

const decode = (s) => s
  .replace(/&amp;/g, '&').replace(/&quot;/g, '"').replace(/&#0?39;/g, '’').replace(/&apos;/g, '’')
  .replace(/&lt;/g, '<').replace(/&gt;/g, '>').replace(/&nbsp;/g, ' ').replace(/&reg;/g, '®').replace(/&trade;/g, '™')
  .replace(/&#(\d+);/g, (_, n) => String.fromCodePoint(+n));
const text = (s) => decode(s.replace(/<br\s*\/?>/gi, '\n').replace(/<[^>]+>/g, '')).replace(/[ \t]+/g, ' ').replace(/ *\n */g, '\n').trim();
const sentence = (s) => (s === s.toUpperCase() && /[A-Z]/.test(s) ? s.charAt(0) + s.slice(1).toLowerCase() : s);

// Steam's description HTML → a flat list of blocks the designs render without innerHTML.
function blocks(html) {
  const out = [];
  const media = /<video[^>]*poster="([^"]+)"[^>]*>\s*<source src="([^"]+)"[\s\S]*?<\/video>|<img[^>]*src="([^"]+)"[^>]*>/gi;
  const pushText = (chunk) => {
    const re = /<h[1-3][^>]*>([\s\S]*?)<\/h[1-3]>|<ul[^>]*>([\s\S]*?)<\/ul>/gi;
    let last = 0, m;
    const para = (s) => {
      for (const p of s.split(/<\/p>|<br\s*\/?>\s*<br\s*\/?>/i)) {
        const t = text(p);
        if (t) out.push({ t: 'p', text: t.replace(/\n+/g, ' ') });
      }
    };
    while ((m = re.exec(chunk))) {
      para(chunk.slice(last, m.index));
      if (m[1] != null) { const t = text(m[1]); if (t) out.push({ t: 'h', text: sentence(t) }); }
      else {
        const items = [...m[2].matchAll(/<li[^>]*>([\s\S]*?)<\/li>/gi)].map((x) => text(x[1]).replace(/\n+/g, ' ')).filter(Boolean);
        if (items.length) out.push({ t: 'ul', items });
      }
      last = re.lastIndex;
    }
    para(chunk.slice(last));
  };
  let last = 0, m;
  while ((m = media.exec(html))) {
    pushText(html.slice(last, m.index));
    if (m[1]) out.push({ t: 'video', poster: decode(m[1]), src: decode(m[2]) });
    else out.push({ t: 'img', src: decode(m[3]) });
    last = media.lastIndex;
  }
  pushText(html.slice(last));
  return out;
}

async function achievements(appid) {
  const [page, pct] = await Promise.all([
    fetch(`https://steamcommunity.com/stats/${appid}/achievements/?l=english`).then((r) => r.text()).catch(() => ''),
    fetch(`https://api.steampowered.com/ISteamUserStats/GetGlobalAchievementPercentagesForApp/v2/?gameid=${appid}`).then((r) => r.json()).catch(() => null),
  ]);
  const src = page.replace(/\\"/g, '"').replace(/\\\//g, '/');
  const re = /\{"internal_key":\d+,"api_name":"(.*?)","name":"(.*?)","description":"(.*?)","hidden":(true|false),"archived":(true|false),"icon_achieved":"(.*?)","icon_unachieved":"(.*?)"/g;
  const pcts = Object.fromEntries((pct?.achievementpercentages?.achievements || []).map((a) => [a.name, +a.percent]));
  const seen = new Set();
  const all = [];
  let m;
  while ((m = re.exec(src))) {
    if (seen.has(m[1]) || m[5] === 'true') continue;
    seen.add(m[1]);
    const hash = (u) => u.split('/').pop();
    const js = (x) => { try { return JSON.parse(`"${x}"`); } catch { return x.replace(/\\u([0-9a-f]{4})/gi, (_, h) => String.fromCharCode(parseInt(h, 16))); } };
    all.push({ id: m[1], name: decode(js(m[2])), desc: decode(js(m[3])), hidden: m[4] === 'true', icon: hash(m[6]), gray: hash(m[7]), pct: pcts[m[1]] ?? null });
  }
  return { total: all.length, list: all.slice(0, MAX_ACH) };
}

const pages = {};
for (const g of GAMES) {
  const appid = g.appid || EXTRA[g.id];
  if (!appid) continue;
  const r = await fetch(`https://store.steampowered.com/api/appdetails?appids=${appid}&l=english`).then((x) => x.json());
  const d = r?.[appid]?.data;
  if (!d) { console.warn('no store data', g.id); continue; }
  const ach = await achievements(appid);
  pages[g.id] = {
    appid,
    about: blocks(d.about_the_game || d.detailed_description || ''),
    shots: (d.screenshots || []).slice(0, MAX_SHOTS).map((s) => ({ thumb: s.path_thumbnail, full: s.path_full })),
    genres: (d.genres || []).map((x) => x.description),
    released: d.release_date?.date || null,
    publisher: d.publishers?.[0] || null,
    metacritic: d.metacritic?.score || null,
    features: (d.categories || []).map((x) => x.description).slice(0, 8),
    ach,
  };
  console.log(g.id, pages[g.id].about.length, 'blocks', pages[g.id].shots.length, 'shots', ach.total, 'achievements');
  await new Promise((res) => setTimeout(res, 400));
}
writeFileSync(new URL('../src/core/storePages.json', import.meta.url), JSON.stringify(pages));
