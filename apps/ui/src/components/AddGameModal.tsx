import React, { useState } from 'react';
import { Effect } from 'effect';
import { ChevronDown } from 'lucide-react';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel, cx } from './ui';
import { LauncherApi } from '../effect/runtime';

/** Suggest a title from the executable's file name: the base without its
 *  extension, underscores turned into spaces. */
export function suggestTitle(exe: string): string {
  const base = exe.split('/').pop() ?? exe;
  const noExt = base.includes('.') ? base.slice(0, base.lastIndexOf('.')) : base;
  return noExt.replace(/[_]+/g, ' ').trim();
}

/** Advanced sideload options, stored in the new game's GameSettings. */
export interface AddGameOptions {
  workingDir: string;
  winePrefix: string;
}

export function AddGameModal({ onClose, onAdd }: { onClose: () => void; onAdd: (title: string, exe: string, opts: AddGameOptions) => void }) {
  const [title, setTitle] = useState('');
  const [exe, setExe] = useState('');
  const [workingDir, setWorkingDir] = useState('');
  const [winePrefix, setWinePrefix] = useState('');
  const [advancedOpen, setAdvancedOpen] = useState(false);
  const [workError, setWorkError] = useState<string | null>(null);
  const [prefixError, setPrefixError] = useState<string | null>(null);
  const valid = title.trim() && exe.trim();
  const advancedSet = (workingDir.trim() ? 1 : 0) + (winePrefix.trim() ? 1 : 0);

  const browseExe = () => {
    Effect.runPromise(LauncherApi.pickGameExecutable(exe.trim()))
      .then((picked) => {
        if (!picked) return; // cancelled: leave the field alone
        setExe(picked);
        if (!title.trim()) {
          const suggestion = suggestTitle(picked);
          if (suggestion) setTitle(suggestion);
        }
      })
      .catch(() => {}); // no picker here: the text field still works
  };

  const browseWork = () => {
    Effect.runPromise(LauncherApi.pickFolder('Choose the working directory', workingDir.trim() || exe.trim()))
      .then((picked) => {
        if (!picked) return;
        setWorkingDir(picked);
        setWorkError(null);
      })
      .catch(() => {});
  };

  const browsePrefix = () => {
    Effect.runPromise(LauncherApi.pickFolder('Choose the Wine prefix folder', winePrefix.trim() || workingDir.trim() || exe.trim()))
      .then((picked) => {
        if (!picked) return;
        setWinePrefix(picked);
        setPrefixError(null);
      })
      .catch(() => {});
  };

  const submit = () => {
    const work = workingDir.trim();
    if (work && !work.startsWith('/')) {
      setWorkError('The working directory must be an absolute path.');
      return;
    }
    const prefix = winePrefix.trim();
    if (prefix && !prefix.startsWith('/') && !prefix.startsWith('~/')) {
      setPrefixError('The Wine prefix must be an absolute path.');
      return;
    }
    if (valid) onAdd(title.trim(), exe.trim(), { workingDir: work, winePrefix: prefix });
  };

  return (
    <Modal label="Add game" onClose={onClose} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          submit();
        }}
      >
        <h2 className="text-[20px] font-extrabold tracking-tight">Add game</h2>
        <p className="mt-1 text-[12.5px] text-white/70">Sideload a game by its executable path. It's saved to your library and runs through Platypus's runner pipeline.</p>
        <label className="mt-5 block">
          <MonoLabel className="mb-1.5">title</MonoLabel>
          <Input autoFocus value={title} onChange={(e) => setTitle(e.target.value)} placeholder="My Game" />
        </label>
        <div className="mt-4">
          <MonoLabel className="mb-1.5">executable</MonoLabel>
          <div className="flex items-center gap-2">
            <Input
              value={exe}
              onChange={(e) => setExe(e.target.value)}
              placeholder="/home/you/Games/game/game.exe"
              aria-label="executable"
              className="min-w-0 flex-1"
            />
            <Button type="button" variant="neutral" onClick={browseExe} className="flex-none">
              Browse…
            </Button>
          </div>
        </div>
        <div className="mt-4 border-t border-white/[0.07] pt-3">
          <button
            type="button"
            aria-expanded={advancedOpen}
            onClick={() => setAdvancedOpen((o) => !o)}
            className="flex items-center gap-1.5 font-mono text-[12px] text-white/60 transition hover:text-white/85"
          >
            <ChevronDown className={cx('size-3.5 transition-transform', advancedOpen ? '' : '-rotate-90')} aria-hidden />
            Advanced
            {advancedSet > 0 && <span className="tabular-nums text-amber-200/90">· {advancedSet} set</span>}
          </button>
          <div hidden={!advancedOpen} className="mt-4 space-y-5">
            <div>
              <MonoLabel className="mb-1.5">working directory <span className="normal-case tracking-normal text-white/55">· optional</span></MonoLabel>
              <div className="flex items-center gap-2">
                <Input
                  value={workingDir}
                  onChange={(e) => { setWorkingDir(e.target.value); setWorkError(null); }}
                  placeholder="The executable's folder"
                  aria-label="working directory"
                  className="min-w-0 flex-1"
                />
                <Button type="button" variant="neutral" onClick={browseWork} className="flex-none">
                  Browse…
                </Button>
              </div>
              <p className="mt-1.5 text-[12px] leading-relaxed text-white/70">Where the game runs. Empty means the executable's folder.</p>
              {workError && <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{workError}</p>}
            </div>
            <div>
              <MonoLabel className="mb-1.5">wine prefix <span className="normal-case tracking-normal text-white/55">· optional</span></MonoLabel>
              <div className="flex items-center gap-2">
                <Input
                  value={winePrefix}
                  onChange={(e) => { setWinePrefix(e.target.value); setPrefixError(null); }}
                  placeholder="The default prefix folder"
                  aria-label="wine prefix"
                  className="min-w-0 flex-1"
                />
                <Button type="button" variant="neutral" onClick={browsePrefix} className="flex-none">
                  Browse…
                </Button>
              </div>
              <p className="mt-1.5 text-[12px] leading-relaxed text-white/70">The Wine prefix the game runs in. Empty falls back to the default prefix folder; the folder is created on demand.</p>
              {prefixError && <p role="alert" className="mt-1 font-mono text-hint text-orange-300">{prefixError}</p>}
            </div>
          </div>
        </div>
        <div className="mt-6 flex justify-end gap-2.5">
          <Button variant="neutral" onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" disabled={!valid}>Add game</Button>
        </div>
      </form>
    </Modal>
  );
}
