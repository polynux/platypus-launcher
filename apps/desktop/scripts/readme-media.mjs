// `node apps/desktop/scripts/readme-media.mjs [--port 9222] [--game <title>] [--out docs/media]`
// captures the README's screenshots and tour GIF from a running app.
//
// Start the app with remote debugging first, on your real library:
//   PLATYPUS_REMOTE_DEBUGGING_PORT=9222 bun run start
// Views are reached by position in the sidebar nav and the library grid, so
// any UI language works. --game opens that game's page (title substring),
// else the first card of the library. The viewport is pinned to 1440x900
// while capturing and restored afterwards. The GIF needs ffmpeg on PATH.
import { execFileSync } from 'node:child_process';
import { mkdirSync, mkdtempSync, rmSync, writeFileSync } from 'node:fs';
import { tmpdir } from 'node:os';
import { join, resolve } from 'node:path';
import { Cdp } from '../e2e/cdp.mjs';

const arg = (name, def) => {
  const i = process.argv.indexOf(`--${name}`);
  return i > 0 ? process.argv[i + 1] : def;
};
const port = Number(arg('port', process.env.PLATYPUS_REMOTE_DEBUGGING_PORT ?? 9222));
const game = arg('game', '');
const out = resolve(arg('out', 'docs/media'));
const W = 1440;
const H = 900;
const sleep = (ms) => new Promise((r) => setTimeout(r, ms));

const d = await Cdp.connect(port);
await d.send('Emulation.setDeviceMetricsOverride', { width: W, height: H, deviceScaleFactor: 1, mobile: false });

// Sidebar nav order: dashboard, library, downloads, activity.
const nav = (i) => d.evaluate((i) => {
  const btns = [...(document.querySelector('nav')?.querySelectorAll('button') ?? [])];
  if (!btns[i]) throw new Error(`sidebar nav item ${i} missing`);
  btns[i].click();
}, [i]);
const openGame = () => d.evaluate((title) => {
  const cards = [...document.querySelectorAll('button.card-compat')];
  const hit = title ? cards.find((c) => (c.textContent ?? '').toLowerCase().includes(title.toLowerCase())) : cards[0];
  if (!hit) throw new Error(title ? `no library card matches "${title}"` : 'library is empty');
  hit.click();
}, [game]);
const scroll = (y) => d.evaluate((y) => {
  const el = [...document.querySelectorAll('main, main *')].find((e) => e.scrollHeight > e.clientHeight + 50 && /(auto|scroll)/.test(getComputedStyle(e).overflowY));
  (el ?? document.scrollingElement).scrollTo({ top: y, behavior: 'smooth' });
}, [y]);

// Covers load lazily: give each view time to settle before a shot.
async function shot(name) {
  await sleep(2500);
  await d.screenshot(join(out, `${name}.png`));
  console.log(`readme-media: ${name}.png`);
}

// The tour: the same walk, recorded through the screencast.
async function tour() {
  const frames = mkdtempSync(join(tmpdir(), 'platypus-tour-'));
  const list = [];
  d.ws.addEventListener('message', (m) => {
    const msg = JSON.parse(m.data);
    if (msg.method !== 'Page.screencastFrame') return;
    const f = join(frames, `${String(list.length).padStart(5, '0')}.jpg`);
    writeFileSync(f, Buffer.from(msg.params.data, 'base64'));
    list.push({ f, t: msg.params.metadata.timestamp });
    d.send('Page.screencastFrameAck', { sessionId: msg.params.sessionId }).catch(() => {});
  });
  await nav(0);
  await sleep(1500);
  await d.send('Page.startScreencast', { format: 'jpeg', quality: 85, maxWidth: W, maxHeight: H });
  await sleep(2000);
  await nav(1);
  await sleep(2000);
  await scroll(600);
  await sleep(1500);
  await scroll(0);
  await sleep(800);
  await openGame();
  await sleep(2500);
  await scroll(700);
  await sleep(2000);
  await nav(0);
  await sleep(1500);
  await d.send('Page.stopScreencast');
  // The screencast only sends frames on change: hold each one until the next.
  const concat = list.map((x, i) => `file '${x.f}'\nduration ${((list[i + 1]?.t ?? x.t + 1) - x.t).toFixed(3)}`).join('\n');
  writeFileSync(join(frames, 'list.txt'), `${concat}\nfile '${list.at(-1).f}'\n`);
  execFileSync('ffmpeg', ['-y', '-loglevel', 'error', '-f', 'concat', '-safe', '0', '-i', join(frames, 'list.txt'),
    '-vf', 'fps=10,scale=960:-1:flags=lanczos,split[a][b];[a]palettegen=max_colors=192[p];[b][p]paletteuse=dither=sierra2_4a',
    join(out, 'tour.gif')]);
  rmSync(frames, { recursive: true, force: true });
  console.log(`readme-media: tour.gif (${list.length} frames)`);
}

try {
  mkdirSync(out, { recursive: true });
  await nav(0);
  await shot('dashboard');
  await nav(1);
  await shot('library');
  await openGame();
  await shot('game');
  await tour();
} finally {
  await d.send('Emulation.clearDeviceMetricsOverride').catch(() => {});
  d.close();
}
process.exit(0);
