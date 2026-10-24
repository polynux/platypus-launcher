// The native confirmation for ApprovePlugin (issue #89): approving a plugin
// lets it run code on the machine, so the page may request approval but
// never grant it. Main looks the plugin up on the core, shows a native
// dialog parented to the window, and forwards to the core only on an
// explicit Approve. The default button is Cancel.
import { shellText } from './locale.ts';

/** The plugin as the confirmation names it (internal/types.StorePlugin). */
export interface ApprovalPlugin {
  id: string;
  name: string;
  dir: string;
  install_folder?: string;
  declared?: string[] | null;
  network?: string[] | null;
  read?: string[] | null;
  write?: string[] | null;
}

/** The coded refusal the bridge answers when the user declines. */
export const APPROVAL_DECLINED_CODE = 'plugin.approvalDeclined';

/**
 * The test-only answer for the native dialog. Tests and e2e set
 * PLATYPUS_TEST_APPROVE_DIALOG to approve/decline in the main process's
 * environment; the page cannot reach it (it never crosses the bridge), and
 * production never sets it, so the dialog always shows there.
 */
export function testApproveAnswer(env: NodeJS.ProcessEnv = process.env): 'approve' | 'decline' | undefined {
  const v = env.PLATYPUS_TEST_APPROVE_DIALOG;
  return v === 'approve' || v === 'decline' ? v : undefined;
}

/** The plugin of the approval request, by id ('' when the core knows none). */
export function findApprovalPlugin(plugins: ApprovalPlugin[], id: unknown): ApprovalPlugin | undefined {
  if (typeof id !== 'string' || id === '') return undefined;
  return plugins.find((p) => p?.id === id);
}

/** The dialog's message: the plugin's name and id, in the shell language. */
export function approvalMessage(plugin: ApprovalPlugin, lang: string): string {
  return shellText(lang, 'approvePluginMessage', { name: plugin.name || plugin.id, id: plugin.id });
}

function list(items: string[] | null | undefined, empty: string): string {
  return items && items.length > 0 ? items.join(', ') : empty;
}

/** The dialog's detail: the folder and the declared permissions. */
export function approvalDetail(plugin: ApprovalPlugin, lang: string): string {
  const folder = plugin.install_folder || plugin.dir || '';
  const lines = [
    `${shellText(lang, 'approvePluginFolder')}: ${folder}`,
    `${shellText(lang, 'approvePluginPermissions')}: ${list(plugin.declared, shellText(lang, 'approvePluginNone'))}`,
    `${shellText(lang, 'approvePluginNetwork')}: ${list(plugin.network, shellText(lang, 'approvePluginNoHosts'))}`,
    `${shellText(lang, 'approvePluginReads')}: ${list(plugin.read, shellText(lang, 'approvePluginNoPaths'))}`,
    `${shellText(lang, 'approvePluginWrites')}: ${list(plugin.write, shellText(lang, 'approvePluginNoPaths'))}`,
  ];
  return lines.join('\n');
}

/** The bridge reply when the user declines: a coded error the UI translates. */
export function declinedReply(name: string): { ok: false; error: string; data: { code: string; params: { name: string } } } {
  return {
    ok: false,
    error: `Approval for ${name} was declined`,
    data: { code: APPROVAL_DECLINED_CODE, params: { name } },
  };
}

export interface ApproveGate {
  listPlugins: () => Promise<ApprovalPlugin[]>;
  /** The native dialog: true only on an explicit Approve. */
  showDialog: (plugin: ApprovalPlugin) => Promise<boolean>;
  /** Forwards the request to the core. */
  forward: (args: unknown[]) => Promise<unknown>;
  /** The test-only answer, when set, replaces the native dialog. */
  testAnswer?: 'approve' | 'decline';
}

/**
 * Runs the approval gate for one bridge call: looks the plugin up, answers
 * from the test hook when one is set, otherwise asks the native dialog, and
 * forwards to the core only on approval. A decline never reaches the core.
 */
export async function runApproveGate(
  args: unknown[],
  gate: ApproveGate,
): Promise<{ ok: true; result: unknown } | { ok: false; error: string; data?: { code: string; params?: Record<string, string> } }> {
  const id = Array.isArray(args) && typeof args[0] === 'string' ? args[0] : '';
  let plugins: ApprovalPlugin[] = [];
  try {
    plugins = await gate.listPlugins();
  } catch (err) {
    return { ok: false, error: err instanceof Error ? err.message : String(err) };
  }
  const plugin = findApprovalPlugin(plugins, id) ?? { id, name: id, dir: '' };
  if (gate.testAnswer === 'approve') return { ok: true, result: await gate.forward(args) };
  if (gate.testAnswer === 'decline') return declinedReply(plugin.name || plugin.id);
  if (await gate.showDialog(plugin)) return { ok: true, result: await gate.forward(args) };
  return declinedReply(plugin.name || plugin.id);
}
