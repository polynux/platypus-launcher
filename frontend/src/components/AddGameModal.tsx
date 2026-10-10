import React, { useState } from 'react';
import { Modal } from './Modal';
import { Button } from '@/components/ds/button';
import { Input } from '@/components/ds/input';
import { MonoLabel } from './ui';

export function AddGameModal({ onClose, onAdd }: { onClose: () => void; onAdd: (title: string, exe: string) => void }) {
  const [title, setTitle] = useState('');
  const [exe, setExe] = useState('');
  const valid = title.trim() && exe.trim();

  return (
    <Modal label="Add game" onClose={onClose} size="md">
      <form
        onSubmit={(e) => {
          e.preventDefault();
          if (valid) onAdd(title.trim(), exe.trim());
        }}
      >
        <h2 className="text-[20px] font-extrabold tracking-tight">Add game</h2>
        <p className="mt-1 text-[12.5px] text-white/50">Sideload a game by its executable path. It's saved to your library and runs through Platypus's runner pipeline.</p>
        <label className="mt-5 block">
          <MonoLabel className="mb-1.5">title</MonoLabel>
          <Input autoFocus value={title} onChange={(e) => setTitle(e.target.value)} placeholder="My Game" />
        </label>
        <label className="mt-4 block">
          <MonoLabel className="mb-1.5">executable</MonoLabel>
          <Input value={exe} onChange={(e) => setExe(e.target.value)} placeholder="/home/you/Games/game/game.exe" />
        </label>
        <div className="mt-6 flex justify-end gap-2.5">
          <Button variant="neutral" onClick={onClose}>Cancel</Button>
          <Button type="submit" variant="primary" disabled={!valid}>Add game</Button>
        </div>
      </form>
    </Modal>
  );
}
