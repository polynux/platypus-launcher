import { defineConfig } from 'vitest/config';
import react from '@vitejs/plugin-react';
import path from 'node:path';

const slow = process.env.PLATYPUS_TEST_SLOW === '1';

export default defineConfig({
  plugins: [react()],
  resolve: { alias: { '@': path.resolve(__dirname, './src') } },
  test: {
    environment: 'jsdom',
    setupFiles: ['./src/test/setup.ts'],
    globals: true,
    // One jsdom per worker instead of one per file (issue #40): 29 s → 13 s
    // for the same 642 tests, keeping per-file isolation.
    pool: 'vmThreads',
    // Slow integration tests live in *.slow.test.{ts,tsx} and only run with
    // PLATYPUS_TEST_SLOW=1 (issue #40). There are currently none.
    exclude: slow
      ? ['**/node_modules/**', '**/dist/**']
      : ['**/node_modules/**', '**/dist/**', '**/*.slow.test.{ts,tsx}'],
  },
});
