import React, { useState } from 'react';
import { Modal } from './Modal';
import { MonoLabel, btnNeutral, btnPrimary, cx } from './ui';

const inputClass =
  'w-full rounded-xl bg-white/[0.06] px-3.5 py-2.5 text-[12.5px] text-white placeholder-white/30 outline-none ring-amber-300/50 focus:ring-1';

export function AddGameModal({ onClose, onAdd }: { onClose: () => void; onAdd: (title: string, exe: string) => void }) {
  const [title, setTitle] = useState('');
  const [exe, setExe] = useState('');
  const valid = title.trim() && exe.trim();

  return (
    <Modal label="Add game" onClose={onClose} className="max-w-[520px]">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (valid) onAdd(title.trim(), exe.trim());
        }}
      >
        <h2 className="text-[20px] font-extrabold tracking-tight">Add game</h2>
        <p className="mt-1 text-[12.5px] text-white/50">Sideload a Windows executable. Kept for this session only, until custom games are saved by the backend.</p>
        <label className="mt-5 block">
          <MonoLabel className="mb-1.5">title</MonoLabel>
          <input autoFocus value={title} onChange={(e) => setTitle(e.target.value)} placeholder="My Game" className={inputClass} />
        </label>
        <label className="mt-4 block">
          <MonoLabel className="mb-1.5">executable</MonoLabel>
          <input value={exe} onChange={(e) => setExe(e.target.value)} placeholder="/home/you/Games/game/game.exe" className={cx(inputClass, 'font-mono text-[11.5px]')} />
        </label>
        <div className="mt-6 flex justify-end gap-2.5">
          <button type="button" onClick={onClose} className={cx(btnNeutral, 'px-4 py-2.5 text-[12.5px]')}>Cancel</button>
          <button type="submit" disabled={!valid} className={cx(btnPrimary, 'px-5 py-2.5 text-[12.5px] disabled:cursor-not-allowed')}>Add game</button>
        </div>
      </form>
    </Modal>
  );
}
