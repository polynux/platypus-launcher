import React, { useState } from 'react';
import { Effect } from 'effect';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel } from './ui';
import { LauncherApi } from '../effect/runtime';

/** Suggest a title from the executable's file name: the base without its
 *  extension, underscores turned into spaces. */
export function suggestTitle(exe: string): string {
  const base = exe.split('/').pop() ?? exe;
  const noExt = base.includes('.') ? base.slice(0, base.lastIndexOf('.')) : base;
  return noExt.replace(/[_]+/g, ' ').trim();
}

export function AddGameModal({ onClose, onAdd }: { onClose: () => void; onAdd: (title: string, exe: string) => void }) {
  const [title, setTitle] = useState('');
  const [exe, setExe] = useState('');
  const valid = title.trim() && exe.trim();

  const browse = () => {
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

  return (
    <Modal label="Add game" onClose={onClose} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (valid) onAdd(title.trim(), exe.trim());
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
            <Button type="button" variant="neutral" onClick={browse} className="flex-none">
              Browse…
            </Button>
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
