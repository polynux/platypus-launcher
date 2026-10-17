// E2E scenarios (issue #69): four step lists against the sandbox app.
// Each scenario is { name, steps: [{ name, run(ctx) }] }. A failing step
// names itself and keeps a screenshot; run.mjs enforces that. Steps drive
// the UI through the CDP driver where the words are stable (text, roles,
// aria labels) and use the page's own backend (window.platypus.invoke, the
// same RPCs the UI uses) for setup and strict checks. No new abstraction:
// a scenario is just data plus small async functions.
//
// Adding one: append { name, steps } to SCENARIOS, keep each step to one
// visible action plus one check, and prefer waiting for words the user sees
// (waitForText) over selectors. See docs/testing.md.
import { readFileSync, existsSync } from 'node:fs';

// The backend the page itself uses: window.platypus.invoke(method, args).
// Same RPCs, same args as apps/ui/src/effect/runtime.ts.
async function invoke(driver, method, args = []) {
  return driver.evaluate(async (m, a) => {
    // @ts-ignore: page global from the desktop preload
    return await window.platypus.invoke(m, a);
  }, [method, args]);
}

async function library(driver) {
  const lib = await invoke(driver, 'GetLibrary', []);
  return lib?.games ?? lib ?? [];
}

async function collections(driver) {
  try {
    return await invoke(driver, 'ListCollections', []);
  } catch {
    return [];
  }
}

function findGame(games, idPart) {
  return games.find((g) => g.id === idPart || g.id?.endsWith?.('.' + idPart) || g.title?.includes(idPart));
}

// Open the library view through the sidebar nav (exact label match: the
// dashboard also has a "Full library" link that contains the word).
async function gotoLibrary(driver) {
  await driver.waitForText('Library', { timeoutMs: 60000 });
  await driver.evaluate(() => {
    const nav = document.querySelector('nav[aria-label="Main"]');
    const btns = [...(nav?.querySelectorAll('button') ?? document.querySelectorAll('button'))];
    const hit = btns.find((b) => (b.textContent ?? '').trim().toLowerCase().startsWith('library'));
    if (!hit) throw new Error('sidebar Library nav missing');
    hit.click();
  });
  await driver.waitForText('All titles', { timeoutMs: 60000 });
  // A previous scenario's search persists in the library view: clear it so
  // each scenario starts from the full listing.
  try {
    await driver.evaluate(() => {
      const input = document.querySelector('input[aria-label="Filter library"]');
      if (input && input.value) {
        input.focus();
        const setter = Object.getOwnPropertyDescriptor(HTMLInputElement.prototype, 'value')?.set;
        if (setter) setter.call(input, '');
        else input.value = '';
        input.dispatchEvent(new Event('input', { bubbles: true }));
        input.dispatchEvent(new Event('change', { bubbles: true }));
      }
    });
  } catch {}
}

// The UI loads collections once and updates them through its dialogs: a
// collection created behind its back (invoke, the same RPC the dialog uses)
// reaches the filter dropdown and game page only after a reload.
async function reloadAndGotoLibrary(driver) {
  await driver.evaluate(() => location.reload());
  await gotoLibrary(driver);
}

// Open a game's page: library first, then search and click its card.
// Fixtures start uninstalled, so the owned view must be on to find them.
// The Filters panel toggles, so open it only when the owned segment is not
// already visible instead of clicking blindly.
async function openGamePage(driver, title) {
  await gotoLibrary(driver);
  const panelOpen = async () =>
    driver.evaluate(() => [...document.querySelectorAll('button')].some((b) => (b.textContent ?? '').trim().toLowerCase() === 'owned'));
  if (!(await panelOpen())) {
    try { await driver.clickText('Filters', { timeoutMs: 8000 }); } catch {}
    await driver.waitFor(async () => [...document.querySelectorAll('button')].some((b) => (b.textContent ?? '').trim().toLowerCase() === 'owned'), [], { label: 'owned segment', timeoutMs: 15000 });
  }
  await driver.evaluate(() => {
    const hit = [...document.querySelectorAll('button')].find((b) => (b.textContent ?? '').trim().toLowerCase() === 'owned');
    if (hit) hit.click();
  });
  await new Promise((r) => setTimeout(r, 800));
  // The library search filters as you type (aria-label "Filter library").
  try {
    await driver.evaluate(() => {
      const input = document.querySelector('input[aria-label="Filter library"]');
      if (input) { input.focus(); input.value = ''; input.dispatchEvent(new Event('input', { bubbles: true })); }
    });
    await driver.type('input[aria-label="Filter library"]', title);
    await new Promise((r) => setTimeout(r, 800));
  } catch {}
  // Click the card whose title matches (cards render the title as text).
  await driver.evaluate((t) => {
    const btns = [...document.querySelectorAll('button')];
    const hit = btns.find((b) => (b.textContent ?? '').includes(t));
    if (!hit) throw new Error('game card missing for ' + t);
    hit.scrollIntoView({ block: 'center' });
    hit.click();
  }, [title]);
}

// The hero's Play/Stop buttons say exactly that (game locale play.play /
// play.stop); an includes-match would hit Echo Player or Recently played
// first, so match exactly here.
async function clickHero(driver, label) {
  await driver.waitForText(label, { timeoutMs: 30000 });
  await driver.evaluate((t) => {
    const btns = [...document.querySelectorAll('button')];
    const hit = btns.find((b) => (b.textContent ?? '').trim() === t);
    if (!hit) throw new Error(t + ' button missing');
    hit.scrollIntoView({ block: 'center' });
    hit.click();
  }, [label]);
}
async function ensureInstalled(driver, gameId, title) {
  let games = await library(driver);
  let g = findGame(games, gameId);
  if (g?.is_installed) return g;
  await openGamePage(driver, title);
  // The install button says exactly Install (game locale "install.install");
  // an includes-match would hit the sidebar's Installed collection first, so
  // match exactly here. Echo fixtures offer no options dialog: one click
  // queues the ~1 s job.
  try {
    await driver.waitForText('Install', { timeoutMs: 15000 });
    await driver.evaluate(() => {
      const btns = [...document.querySelectorAll('button')];
      const hit = btns.find((b) => (b.textContent ?? '').trim() === 'Install');
      if (!hit) throw new Error('Install button missing');
      hit.scrollIntoView({ block: 'center' });
      hit.click();
    });
    // Echo fixtures offer install options (language, add-ons): the hero
    // Install opens the options dialog, whose own Install queues the job.
    await new Promise((r) => setTimeout(r, 2000));
    const options = await driver.evaluate(() =>
      (document.body?.innerText ?? '').includes('Install options'));
    if (options) {
      await driver.evaluate(() => {
        const dialogs = [...document.querySelectorAll('[role="dialog"]')];
        const scope = dialogs.length ? dialogs[dialogs.length - 1] : document;
        const btns = [...scope.querySelectorAll('button')];
        const hit = btns.filter((b) => (b.textContent ?? '').trim() === 'Install').pop()
          ?? btns.find((b) => /install/i.test(b.textContent ?? ''));
        if (!hit) throw new Error('Install options confirm missing');
        hit.click();
      });
    }
  } catch (e) {
    // Already installing or already installed: fall through to the poll.
  }
  await driver.waitFor(async (id) => {
    // @ts-ignore
    const lib = await window.platypus.invoke('GetLibrary', []);
    const list = lib?.games ?? [];
    const hit = list.find((x) => x.id === id || (x.id ?? '').endsWith('.' + id));
    return hit?.is_installed ? true : false;
  }, [gameId], { timeoutMs: 120000, pollMs: 1000, label: `${title} installed` });
  games = await library(driver);
  return findGame(games, gameId);
}

export const SCENARIOS = [
  {
    name: 'library-smart',
    steps: [
      {
        name: 'library-loads-seeded-games',
        run: async ({ driver }) => {
          await gotoLibrary(driver);
          // The library opens on installed games; fixtures start uninstalled,
          // so open Filters and switch to owned to see the seeded bulk games
          // plus fixtures.
          try {
            await driver.clickText('Filters', { timeoutMs: 10000 });
            await new Promise((r) => setTimeout(r, 800));
          } catch {}
          try {
            await driver.clickText('owned', { timeoutMs: 10000 });
            await new Promise((r) => setTimeout(r, 1200));
          } catch {}
          // The grid is virtualized: fixtures past the first rows are not in
          // the DOM until filtered. Filter to them by name.
          await driver.type('input[aria-label="Filter library"]', 'sandbox');
          await driver.waitForText('Echo sandbox-run', { timeoutMs: 60000 });
          const games = await library(driver);
          if (games.length < 30) throw new Error(`library has ${games.length} games, want >= 30 seeded`);
          const ids = new Set(games.map((g) => g.id));
          for (const fx of ['sandbox-run', 'sandbox-ok', 'sandbox-fail', 'sandbox-crash']) {
            if (![...ids].some((id) => id === fx || id.endsWith('.' + fx))) {
              throw new Error(`fixture ${fx} missing from library (${games.length} games)`);
            }
          }
        },
      },
      {
        name: 'smart-collection-filters',
        run: async ({ driver }) => {
          // A smart filter on the seeded facts (#45, ECHO_SEED=7): played
          // games are ~3/5 of the bulk library, so the filtered view is
          // smaller but non-empty. Created through the app's own RPC, the
          // same one the manager dialog uses.
          const list = await invoke(driver, 'CreateCollection', ['e2e-played', { played: true }]);
          const smart = list.find((c) => c.name === 'e2e-played');
          if (!smart || !smart.smart) throw new Error('smart collection was not created');
          // Filter the library by it: reload so the UI picks up the new
          // collection, open Filters, pick it.
          await reloadAndGotoLibrary(driver);
          try {
            await driver.clickText('Filters', { timeoutMs: 15000 });
          } catch {}
          await driver.waitFor(async () => !!document.querySelector('[aria-label="Filter by collection"]'), [], { label: 'collection filter', timeoutMs: 15000 });
          await driver.click('[aria-label="Filter by collection"]');
          await driver.waitForText('e2e-played', { timeoutMs: 15000 });
          await driver.clickText('e2e-played');
          await new Promise((r) => setTimeout(r, 1200));
          const shown = await driver.evaluate(() => document.body.innerText);
          if (!shown.includes('shown')) throw new Error('library counts line missing after filtering');
          const initialCount = await driver.evaluate(() => document.body.innerText.match(/(\d+) shown/)?.[1] ?? null);
          if (initialCount == null) throw new Error('could not read the initial filtered library count');
          // Edit the saved condition in the manager. Changing played=true to
          // false must update the currently selected collection immediately.
          await driver.clickText('Manage');
          await driver.click('[aria-label="Edit conditions for e2e-played"]');
          await driver.evaluate(() => {
            const editor = [...document.querySelectorAll('[data-testid="smart-fields"]')].at(-1);
            const played = [...(editor?.querySelectorAll('select') ?? [])].find((x) => x.closest('label')?.innerText.includes('Played'));
            if (!played) throw new Error('played condition control missing');
            const set = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value').set;
            set.call(played, 'no');
            played.dispatchEvent(new Event('change', { bubbles: true }));
          });
          await driver.clickText('Save changes');
          await driver.waitFor((before) => {
            const count = document.body.innerText.match(/(\d+) shown/)?.[1];
            return count && count !== before;
          }, [initialCount], { label: 'edited smart collection count changes' });
          // Restore played=true so the existing fixture exclusion assertions
          // below still exercise the original filter.
          await driver.clickText('Manage');
          await driver.click('[aria-label="Edit conditions for e2e-played"]');
          await driver.evaluate(() => {
            const editor = [...document.querySelectorAll('[data-testid="smart-fields"]')].at(-1);
            const played = [...(editor?.querySelectorAll('select') ?? [])].find((x) => x.closest('label')?.innerText.includes('Played'));
            if (!played) throw new Error('played condition control missing');
            const set = Object.getOwnPropertyDescriptor(HTMLSelectElement.prototype, 'value').set;
            set.call(played, 'yes');
            played.dispatchEvent(new Event('change', { bubbles: true }));
          });
          await driver.clickText('Save changes');
          await driver.waitFor((before) => document.body.innerText.match(/(\d+) shown/)?.[1] === before, [initialCount], { label: 'restored smart collection count' });
          // The smart collection must match something and exclude something:
          // fixtures never played, so at least one is hidden when filtered.
          if (shown.includes('Echo sandbox-run') && shown.includes('Echo sandbox-ok')
            && shown.includes('Echo sandbox-fail') && shown.includes('Echo sandbox-crash')) {
            // All four unplayed fixtures still shown: the filter did nothing.
            const games = await library(driver);
            const played = games.filter((g) => g.last_played_at);
            if (played.length === 0) throw new Error('no seeded game reports last_played; ECHO_SEED=7 not active?');
            throw new Error('smart filter played=true still shows all four unplayed fixtures');
          }
          // Back to all games for the next scenarios.
          await driver.click('[aria-label="Filter by collection"]');
          await driver.waitForText('All games', { timeoutMs: 15000 });
          await driver.clickText('All games');
          // Clean up so reruns start fresh (delete is idempotent here).
          try { await invoke(driver, 'DeleteCollection', [smart.id]); } catch {}
        },
      },
    ],
  },
  {
    name: 'collection-restart',
    steps: [
      {
        name: 'add-game-to-collection',
        run: async ({ driver }) => {
          await ensureInstalled(driver, 'sandbox-ok', 'Echo sandbox-ok');
          await openGamePage(driver, 'Echo sandbox-ok');
          await driver.waitForText('Collections', { timeoutMs: 30000 });
          // The game page's collections section offers "Add to collection".
          await driver.clickText('Add to collection', { timeoutMs: 15000 });
          await driver.waitForText('Collections for', { timeoutMs: 15000 });
          // Make a manual collection taking this game (same RPC the dialog uses).
          let list = await collections(driver);
          let manual = list.find((c) => c.name === 'e2e-keep' && !c.smart);
          if (!manual) {
            list = await invoke(driver, 'CreateCollection', ['e2e-keep', {}]);
            manual = list.find((c) => c.name === 'e2e-keep');
          }
          if (!manual) throw new Error('manual collection e2e-keep missing');
          const games = await library(driver);
          const g = findGame(games, 'sandbox-ok');
          await invoke(driver, 'AddCollectionGame', [manual.id, g.id]);
          // Close the picker, reload so the game page picks up the new
          // membership, and check the page names the collection.
          await driver.evaluate(() => {
            const btns = [...document.querySelectorAll('button')];
            const done = btns.find((b) => (b.textContent ?? '').trim() === 'Done');
            if (done) done.click();
          });
          await new Promise((r) => setTimeout(r, 1000));
          await reloadAndGotoLibrary(driver);
          await openGamePage(driver, 'Echo sandbox-ok');
          await driver.waitForText('e2e-keep', { timeoutMs: 15000 });
        },
      },
      {
        name: 'collection-survives-restart',
        // The runner stops the app and starts it again on the same sandbox
        // dir (see run.mjs); this step only verifies the collection is still
        // there. It runs twice: before and after the restart.
        run: async ({ driver }) => {
          // Straight invoke, not collections(): a failing call must fail
          // the step with its own error, not read as "no collections".
          const list = await invoke(driver, 'ListCollections', []);
          const manual = list.find((c) => c.name === 'e2e-keep' && !c.smart);
          if (!manual) throw new Error('e2e-keep gone (before or after restart)');
          const games = await library(driver);
          const g = findGame(games, 'sandbox-ok');
          if (!g) throw new Error('sandbox-ok gone after restart');
          if (!manual.games?.includes(g.id)) {
            throw new Error(`e2e-keep no longer lists ${g.id}: ${(manual.games ?? []).join(',')}`);
          }
        },
      },
    ],
  },
  {
    name: 'crash-winetricks',
    steps: [
      {
        name: 'put-crash-game-on-fake-proton',
        run: async ({ driver }) => {
          await ensureInstalled(driver, 'sandbox-crash', 'Echo sandbox-crash');
          // The crash game is a wine fixture (sandbox.sh wine:true), so the
          // runner picker applies. Set it through the app's own per-game
          // setting, like the settings modal does.
          const runners = await invoke(driver, 'GetProtonVersions', []);
          const fake = runners.find((r) => (r.name ?? '').includes('FakeProton-Sandbox'))
            ?? runners.find((r) => (r.id ?? '').includes('FakeProton'));
          if (!fake) throw new Error(`FakeProton-Sandbox missing from runners: ${JSON.stringify(runners.map((r) => r.id ?? r.name))}`);
          const settings = await invoke(driver, 'GetGameSettings', []);
          const games = await library(driver);
          const g = findGame(games, 'sandbox-crash');
          const cur = settings[g.id] ?? settings[g.id?.split?.('.')?.pop?.()] ?? {};
          await invoke(driver, 'SaveGameSettings', [g.id, { ...cur, runner_id: fake.id }]);
          const after = await invoke(driver, 'GetGameSettings', []);
          const saved = after[g.id];
          if (saved?.runner_id !== fake.id) {
            throw new Error(`runner not saved: want ${fake.id}, got ${saved?.runner_id}`);
          }
        },
      },
      {
        name: 'launch-crash-hints-appear',
        run: async ({ driver }) => {
          await openGamePage(driver, 'Echo sandbox-crash');
          await clickHero(driver, 'Play');
          // The fixture sleeps 200 ms, writes vcruntime lines, exits 1: the
          // page shows the crash panel with its winetricks fix (#46).
          await driver.waitForText('vcruntime', { timeoutMs: 60000 });
          await driver.waitForText('winetricks', { timeoutMs: 30000 });
        },
      },
      {
        name: 'winetricks-fix-hits-fake-with-prefix',
        run: async ({ driver, sandboxDir }) => {
          // Click the fix the crash panel offers ("Install vcruntime…").
          const clicked = await driver.evaluate(() => {
            const btns = [...document.querySelectorAll('button')];
            const hit = btns.find((b) => /winetricks/i.test(b.textContent ?? '') && /vcruntime|vcredist|Install/i.test(b.textContent ?? ''));
            if (hit) { hit.click(); return (hit.textContent ?? '').trim(); }
            return null;
          });
          if (!clicked) {
            // Fall back to any winetricks button (the verb is still checked below).
            await driver.clickText('winetricks', { timeoutMs: 15000 });
          }
          await new Promise((r) => setTimeout(r, 3000));
          const logPath = `${sandboxDir}/testbin-calls.log`;
          if (!existsSync(logPath)) throw new Error('testbin log missing: ' + logPath);
          const log = readFileSync(logPath, 'utf8');
          const lines = log.split('\n').filter((l) => l.startsWith('winetricks'));
          if (lines.length === 0) throw new Error('fake winetricks never called; log:\n' + log.slice(-2000));
          const last = lines[lines.length - 1];
          // Proton prefix layout (#46): WINEPREFIX=<prefix>/pfx and the
          // build's own wine, not the system one.
          if (!/\/pfx/.test(last)) throw new Error('winetricks missed the pfx: ' + last);
          if (!/FakeProton-Sandbox/.test(last) && !/files\/bin\/wine/.test(last)) {
            throw new Error('winetricks missed the fake wine: ' + last);
          }
          // The vcruntime log maps to the vcrun2022 verb (internal/crashhints).
          if (!/vcrun|vcredist|vcruntime/.test(last)) throw new Error('winetricks missed the verb: ' + last);
        },
      },
    ],
  },
  {
    name: 'playtime-stop',
    steps: [
      {
        name: 'launch-long-running-game',
        run: async ({ driver }) => {
          await ensureInstalled(driver, 'sandbox-run', 'Echo sandbox-run');
          await openGamePage(driver, 'Echo sandbox-run');
          await clickHero(driver, 'Play');
          // Running state offers Stop (PlayButton); the fixture sleeps until stopped.
          await driver.waitForText('Stop', { timeoutMs: 60000 });
        },
      },
      {
        name: 'stop-and-playtime-updates',
        run: async ({ driver, sandboxDir }) => {
          const beforeList = await library(driver);
          const before = findGame(beforeList, 'sandbox-run');
          const beforeLast = before?.last_played_at ?? 0;
          await clickHero(driver, 'Stop');
          await driver.waitForText('Play', { timeoutMs: 60000 });
          await new Promise((r) => setTimeout(r, 2000));
          const afterList = await library(driver);
          const after = findGame(afterList, 'sandbox-run');
          if (!after) throw new Error('sandbox-run missing after stop');
          // Last played is the fast signal (short sessions keep it but add
          // no minutes: playlog.MinSessionSec=10, minutes round down).
          if (!after.last_played_at || after.last_played_at < beforeLast) {
            throw new Error(`last_played_at did not advance: ${beforeLast} -> ${after.last_played_at}`);
          }
          // The local session log is the minutes behind the UI.
          const sessionsPath = `${sandboxDir}/data/play-sessions.json`;
          if (existsSync(sessionsPath)) {
            const raw = readFileSync(sessionsPath, 'utf8');
            if (!raw.includes('sandbox-run')) throw new Error('play-sessions.json misses sandbox-run');
          }
        },
      },
    ],
  },
];
