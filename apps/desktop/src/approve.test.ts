// ApprovePlugin never reaches the core without the native confirmation
// (issue #89): a decline answers a coded error, an approval forwards once.
import assert from 'node:assert/strict';
import { test } from 'node:test';
import {
  APPROVAL_DECLINED_CODE,
  approvalDetail,
  approvalMessage,
  declinedReply,
  findApprovalPlugin,
  runApproveGate,
  testApproveAnswer,
  type ApprovalPlugin,
  type ApproveGate,
} from './approve.ts';

const plugin: ApprovalPlugin = {
  id: 'echo',
  name: 'Echo',
  dir: '/plugins/echo',
  install_folder: '/games/echo',
  declared: ['library', 'install'],
  network: ['example.com'],
  read: ['/saves'],
  write: ['/games/echo'],
};

function gate(over: Partial<ApproveGate> & { calls?: unknown[][] } = {}): ApproveGate & { calls: unknown[][] } {
  const calls: unknown[][] = over.calls ?? [];
  return {
    listPlugins: async () => [plugin],
    showDialog: async () => {
      throw new Error('dialog must not show when the test hook answers');
    },
    forward: async (args: unknown[]) => {
      calls.push(args);
      return 'approved';
    },
    ...over,
    calls,
  };
}

test('the dialog names the plugin, its folder and its declared permissions', () => {
  assert.match(approvalMessage(plugin, 'en'), /Echo.*echo/);
  const detail = approvalDetail(plugin, 'en');
  for (const want of ['/games/echo', 'library, install', 'example.com', '/saves', '/games/echo']) {
    assert.ok(detail.includes(want), `${want} not in:\n${detail}`);
  }
  assert.match(approvalDetail(plugin, 'fr'), /Dossier/);
});

test('the test hook is env-only: the page cannot reach it', () => {
  assert.equal(testApproveAnswer({} as NodeJS.ProcessEnv), undefined);
  assert.equal(testApproveAnswer({ PLATYPUS_TEST_APPROVE_DIALOG: 'approve' } as NodeJS.ProcessEnv), 'approve');
  assert.equal(testApproveAnswer({ PLATYPUS_TEST_APPROVE_DIALOG: 'decline' } as NodeJS.ProcessEnv), 'decline');
  assert.equal(testApproveAnswer({ PLATYPUS_TEST_APPROVE_DIALOG: 'yes' } as NodeJS.ProcessEnv), undefined);
});

test('a declined approval never reaches the core and carries a coded error', async () => {
  const g = gate({ testAnswer: 'decline' });
  const r = await runApproveGate(['echo'], g);
  assert.equal(g.calls.length, 0);
  assert.equal(r.ok, false);
  if (!r.ok) {
    assert.equal(r.data?.code, APPROVAL_DECLINED_CODE);
    assert.deepEqual(r.data?.params, { name: 'Echo' });
  }
  assert.deepEqual(declinedReply('Echo').data, { code: APPROVAL_DECLINED_CODE, params: { name: 'Echo' } });
  assert.equal(findApprovalPlugin([plugin], 'missing'), undefined);
});

test('an accepted approval reaches the core once', async () => {
  const g = gate({ testAnswer: 'approve' });
  const r = await runApproveGate(['echo'], g);
  assert.deepEqual(r, { ok: true, result: 'approved' });
  assert.deepEqual(g.calls, [['echo']]);
});

test('without the hook the native dialog decides: no means never, yes means once', async () => {
  let seen = 0;
  const no = gate({ testAnswer: undefined, showDialog: async () => (seen++, false) });
  const declined = await runApproveGate(['echo'], no);
  assert.equal(no.calls.length, 0);
  assert.equal(declined.ok, false);
  assert.equal(seen, 1);
  const yes = gate({ testAnswer: undefined, showDialog: async (p) => (assert.equal(p.id, 'echo'), true) });
  const approved = await runApproveGate(['echo'], yes);
  assert.deepEqual(approved, { ok: true, result: 'approved' });
  assert.deepEqual(yes.calls, [['echo']]);
});

test('an unreadable plugin list fails without approving', async () => {
  const g = gate({ listPlugins: async () => Promise.reject(new Error('core down')) });
  const r = await runApproveGate(['echo'], g);
  assert.equal(r.ok, false);
  assert.equal(g.calls.length, 0);
});
