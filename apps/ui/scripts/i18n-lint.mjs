#!/usr/bin/env node
// Hardcoded-string lint for converted areas (issue #39): converted files must
// get user-facing text from the catalogs, so this scans them for JSX text
// nodes and literal string props (aria-label, placeholder, title, label) and
// fails on any. Later threads extend CONVERTED as they convert their area
// (see docs/translating.md). Escape hatch for the rare legitimate literal:
// {/* i18n:allow: <reason> */} on the same or the previous line.
import { readFileSync } from 'node:fs';
import path from 'node:path';

const root = path.resolve(import.meta.dirname, '..', 'src');

// Files fully converted to the catalogs (relative to apps/ui/src). Areas
// converted title-only (their view's h1) join this list when the area's
// phase-2 thread converts the rest; until then their bodies stay English.
const CONVERTED = [
  'App.tsx',
  'components/Sidebar.tsx',
  'components/LauncherSettingsView.tsx',
  'components/GameFoldersSettings.tsx',
];

// Literal text that is not user-facing and may stay inline, per file.
const EXEMPT = new Map([
  // The Platypus "P" tile and the brand name in the sidebar header.
  ['components/Sidebar.tsx', new Set(['P', 'Platypus'])],
]);

// JSX text between a tag end and the next tag start. The `<` must open a real
// tag (</, <name>, <name …) so TypeScript generics (Record<string, …>, <A,>)
// on code lines don't count. Heuristic, documented for that reason.
const TEXT_RE = />([^<>{}\n]*[A-Za-z\u00C0-\u024F\u0370-\u1FFF\u2C00-\uD7FF][^<>{}\n]*)<\/?[A-Za-z][A-Za-z0-9-]*(?=[\s>/=])/g;
const PROP_RE = /(aria-label|placeholder|title|label)="([^"]*[A-Za-zÀ-ÿ][^"]*)"/g;

let violations = 0;
for (const rel of CONVERTED) {
  const src = readFileSync(path.join(root, rel), 'utf8');
  const lines = src.split('\n');
  const exempt = EXEMPT.get(rel) ?? new Set();
  const check = (lineNo, text, kind) => {
    const line = lines[lineNo];
    if (line.includes('i18n:allow') || (lineNo > 0 && lines[lineNo - 1].includes('i18n:allow'))) return;
    if (exempt.has(text.trim())) return;
    console.error(`${rel}:${lineNo + 1}: hardcoded ${kind}: ${JSON.stringify(text.trim().slice(0, 80))}`);
    violations++;
  };
  lines.forEach((line, i) => {
    for (const m of line.matchAll(TEXT_RE)) check(i, m[1], 'JSX text');
    for (const m of line.matchAll(PROP_RE)) check(i, m[2], `${m[1]} prop`);
  });
}

if (violations > 0) {
  console.error(`i18n:lint failed: ${violations} hardcoded string(s) in converted areas`);
  process.exit(1);
}
console.log(`i18n:lint ok: ${CONVERTED.length} converted files free of hardcoded strings`);
