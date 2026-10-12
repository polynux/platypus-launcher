// Suite labels for the UI tests (issue #40), mirroring
// internal/testenv on the Go side.
//
// - Slow tests (heavy integration: real timers, big renders) live in
//   `*.slow.test.{ts,tsx}` and are excluded from the default run (see
//   vitest.config.ts); they run with PLATYPUS_TEST_SLOW=1. There are
//   currently none.
// - Network tests dial non-localhost and are refused by the fetch stub in
//   test/setup.ts unless PLATYPUS_TEST_NETWORK=1. There are currently none.
import { describe, it } from 'vitest';

export function isSlowEnabled(): boolean {
  return process.env.PLATYPUS_TEST_SLOW === '1';
}

export function isNetworkEnabled(): boolean {
  return process.env.PLATYPUS_TEST_NETWORK === '1';
}

// Skip-aware wrappers: the suite is skipped in the default run and runs
// with PLATYPUS_TEST_SLOW=1.
export const describeSlow = (
  ...args: Parameters<typeof describe>
): ReturnType<typeof describe> =>
  (isSlowEnabled() ? describe : describe.skip)(...args);

export const itSlow = (...args: Parameters<typeof it>): ReturnType<typeof it> =>
  (isSlowEnabled() ? it : it.skip)(...args);
