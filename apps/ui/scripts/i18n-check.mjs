#!/usr/bin/env node
// i18n key check (issue #39): reports keys used in code but missing from `en`
// (fails), keys in `en` never used (warning), and per-locale completeness
// (informational). Usage: bun run i18n:check [--locales DIR] [--src DIR]
import { readdirSync, readFileSync, statSync } from 'node:fs';
import path from 'node:path';

const root = path.resolve(import.meta.dirname, '..');
const args = process.argv.slice(2);
const opt = (name, fallback) => {
  const i = args.indexOf(name);
  return i >= 0 && args[i + 1] ? args[i + 1] : fallback;
};
const localesDir = path.resolve(opt('--locales', path.join(root, 'src/locales')));
const srcDir = path.resolve(opt('--src', path.join(root, 'src')));

const PLURAL_SUFFIXES = ['_zero', '_one', '_two', '_few', '_many', '_other'];

function flatten(obj, prefix = '', out = new Map()) {
  for (const [k, v] of Object.entries(obj)) {
    const key = prefix ? `${prefix}.${k}` : k;
    if (v && typeof v === 'object' && !Array.isArray(v)) flatten(v, key, out);
    else out.set(key, v);
  }
  return out;
}

function baseKey(key) {
  for (const s of PLURAL_SUFFIXES) {
    if (key.endsWith(s)) return key.slice(0, -s.length);
  }
  return key;
}

function readCatalog(lang) {
  const out = new Map(); // ns -> Map(flatKey -> value)
  let dir;
  try {
    dir = readdirSync(path.join(localesDir, lang));
  } catch {
    return out;
  }
  for (const f of dir) {
    if (!f.endsWith('.json')) continue;
    const ns = f.slice(0, -5);
    try {
      out.set(ns, flatten(JSON.parse(readFileSync(path.join(localesDir, lang, f), 'utf8'))));
    } catch (err) {
      console.error(`cannot parse ${lang}/${f}: ${err.message}`);
      process.exitCode = 1;
    }
  }
  return out;
}

function walk(dir, files = []) {
  for (const e of readdirSync(dir)) {
    const p = path.join(dir, e);
    if (statSync(p).isDirectory()) {
      if (e === 'locales' || e === 'gen' || e === 'node_modules' || e === 'dist') continue;
      walk(p, files);
    } else if (/\.(ts|tsx)$/.test(e) && !/\.test\.(ts|tsx)$/.test(e) && !/\.slow\.test\.(ts|tsx)$/.test(e)) {
      files.push(p);
    }
  }
  return files;
}

// Fully-qualified keys (docs/translating.md): t('nav:library') or
// t('nav:title.library'). Dynamic keys (t(`downloads:${k}`)) mark their whole
// namespace used; the script cannot verify the dynamic part. Bare keys
// (t('heading')) resolve against the file's useTranslation('ns') default.
const STATIC_RE = /\bt\(\s*(['"])((?:\\\1|(?!\1).)+?)\1/g;
const NS_OPT_RE = /\bt\(\s*(['"])([^'"]+?)\1\s*,\s*\{[^}]*?\bns\s*:\s*(['"])([^'"]+)\3/g;
const DYNAMIC_NS_RE = /\bt\(\s*`([A-Za-z0-9_-]+):\$\{/g;
const HOOK_NS_RE = /useTranslation\(\s*(?:\[([^\]]+)\]|(['"])([^'"]+)\2)?/g;

const used = new Set(); // "ns:base.key"
const dynamicNS = new Set();

for (const f of walk(srcDir)) {
  const src = readFileSync(f, 'utf8');
  const hook = [...src.matchAll(HOOK_NS_RE)];
  // The hook's default namespace: useTranslation('ns') or the first of the list.
  let defNS = '';
  for (const h of hook) {
    if (h[3]) {
      defNS = h[3];
      break;
    }
    if (h[1]) {
      const first = h[1].match(/(['"])([^'"]+)\1/);
      if (first) {
        defNS = first[2];
        break;
      }
    }
  }
  for (const m of src.matchAll(STATIC_RE)) {
    const key = m[2];
    if (key.includes('${')) continue;
    const colon = key.indexOf(':');
    if (colon > 0) used.add(`${key.slice(0, colon)}:${baseKey(key.slice(colon + 1))}`);
    else if (defNS) used.add(`${defNS}:${baseKey(key)}`);
    // Keys without a namespace and without a hook default are skipped: the
    // script cannot tell which namespace they resolve against.
  }
  for (const m of src.matchAll(NS_OPT_RE)) used.add(`${m[4]}:${baseKey(m[2])}`);
  for (const m of src.matchAll(DYNAMIC_NS_RE)) dynamicNS.add(m[1]);
}

const en = readCatalog('en');
const enBases = new Map(); // ns -> Set(base)
for (const [ns, flat] of en) {
  enBases.set(ns, new Set([...flat.keys()].map(baseKey)));
}

let missing = 0;
for (const u of [...used].sort()) {
  const colon = u.indexOf(':');
  const ns = u.slice(0, colon);
  const key = u.slice(colon + 1);
  const bases = enBases.get(ns);
  if (!bases) {
    console.error(`missing namespace in en: ${ns} (used by ${u})`);
    missing++;
  } else if (!bases.has(key)) {
    console.error(`missing key in en: ${u}`);
    missing++;
  }
}

const unused = [];
for (const [ns, bases] of enBases) {
  for (const b of bases) {
    if (dynamicNS.has(ns)) continue;
    if (!used.has(`${ns}:${b}`)) unused.push(`${ns}:${b}`);
  }
}
for (const u of unused.sort()) console.warn(`warning: unused key in en: ${u}`);

// Per-locale completeness (informational).
let langs = [];
try {
  langs = readdirSync(localesDir).filter((e) => statSync(path.join(localesDir, e)).isDirectory());
} catch {
  /* no locales dir in fixture-less runs */
}
for (const lang of langs.sort()) {
  if (lang === 'en' || lang === 'en-XA') continue;
  const catalog = readCatalog(lang);
  let total = 0;
  let have = 0;
  for (const [ns, bases] of enBases) {
    const flat = catalog.get(ns);
    const got = flat ? new Set([...flat.keys()].map(baseKey)) : new Set();
    for (const b of bases) {
      total++;
      if (got.has(b)) have++;
    }
  }
  const pct = total === 0 ? 100 : Math.round((have / total) * 100);
  console.log(`locale ${lang}: ${have}/${total} keys (${pct}%)`);
}

if (missing > 0) {
  console.error(`i18n:check failed: ${missing} missing key(s) in en`);
  process.exit(1);
}
console.log(`i18n:check ok: ${used.size} keys used, all present in en`);
