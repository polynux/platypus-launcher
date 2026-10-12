// Minimal node env typing for test-only files (vitest runs in node, but
// apps/ui has no @types/node dependency).
declare const process: { env: Record<string, string | undefined> };
