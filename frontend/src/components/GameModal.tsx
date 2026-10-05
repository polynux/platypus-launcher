import React, { useState, useEffect } from 'react';
import { 
  X, 
  Play, 
  Terminal, 
  Cpu, 
  ShieldCheck, 
  Sliders, 
  Activity, 
  Wrench, 
  Check, 
  Copy,
  Zap
} from 'lucide-react';
import { Effect } from 'effect';
import type { Game, ProtonVersion } from '../types';
import { LauncherApi } from '../effect/runtime';

interface GameModalProps {
  game: Game | null;
  protonVersions: ProtonVersion[];
  onClose: () => void;
  onSave: (updatedGame: Game) => void;
  onLaunch: (game: Game) => void;
}

export const GameModal: React.FC<GameModalProps> = ({
  game,
  protonVersions,
  onClose,
  onSave,
  onLaunch,
}) => {
  if (!game) return null;

  const [activeTab, setActiveTab] = useState<'general' | 'runner' | 'fixes' | 'advanced'>('general');
  const [settings, setSettings] = useState(game.settings);
  const [cmdPreview, setCmdPreview] = useState<string>('');
  const [copied, setCopied] = useState(false);

  // Update preview command when settings change
  useEffect(() => {
    const updatedGame = { ...game, settings };
    Effect.runPromise(LauncherApi.previewCommand(updatedGame)).then((res) => {
      setCmdPreview(res);
    });
  }, [settings, game]);

  const handleCopyCommand = () => {
    navigator.clipboard.writeText(cmdPreview);
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const applyFixWinetricks = (tricks: string[]) => {
    alert(`Applying winetricks verbs to prefix: ${tricks.join(', ')}`);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/70 backdrop-blur-sm p-6 animate-in fade-in duration-200">
      <div className="bg-zinc-900 border border-zinc-700/80 rounded-2xl w-full max-w-4xl max-h-[90vh] flex flex-col shadow-2xl shadow-purple-950/20 overflow-hidden">
        {/* Header with Banner Image */}
        <div className="relative h-44 bg-zinc-950 flex-shrink-0">
          <img
            src={game.banner_url || game.cover_url}
            alt={game.title}
            className="w-full h-full object-cover opacity-40"
          />
          <div className="absolute inset-0 bg-gradient-to-t from-zinc-900 via-zinc-900/60 to-transparent"></div>
          
          <button
            onClick={onClose}
            className="absolute top-4 right-4 w-8 h-8 rounded-full bg-zinc-800/80 hover:bg-zinc-700 text-zinc-300 flex items-center justify-center transition-colors"
          >
            <X className="w-4 h-4" />
          </button>

          <div className="absolute bottom-4 left-6 right-6 flex items-end justify-between">
            <div>
              <span className="text-xs uppercase font-bold tracking-wider px-2 py-0.5 rounded bg-purple-500/20 text-purple-300 border border-purple-500/30">
                {game.store} Store
              </span>
              <h2 className="text-2xl font-bold text-white mt-1 drop-shadow">
                {game.title}
              </h2>
            </div>
            <button
              onClick={() => onLaunch({ ...game, settings })}
              className="flex items-center gap-2 px-5 py-2.5 rounded-xl bg-purple-600 hover:bg-purple-500 text-white font-medium shadow-lg shadow-purple-600/30 transition-transform active:scale-95"
            >
              <Play className="w-4 h-4 fill-white" />
              <span>Launch Now</span>
            </button>
          </div>
        </div>

        {/* Tab Navigation */}
        <div className="flex border-b border-zinc-800 px-6 bg-zinc-900/50">
          <button
            onClick={() => setActiveTab('general')}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === 'general'
                ? 'border-purple-500 text-purple-400'
                : 'border-transparent text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <Sliders className="w-4 h-4" />
            <span>Launch Options</span>
          </button>
          <button
            onClick={() => setActiveTab('runner')}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === 'runner'
                ? 'border-purple-500 text-purple-400'
                : 'border-transparent text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <Cpu className="w-4 h-4" />
            <span>Proton & Wine</span>
          </button>
          <button
            onClick={() => setActiveTab('fixes')}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === 'fixes'
                ? 'border-purple-500 text-purple-400'
                : 'border-transparent text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <Wrench className="w-4 h-4 text-emerald-400" />
            <span>Fixes & Tweaks</span>
            {game.fix_info && (
              <span className="w-2 h-2 rounded-full bg-emerald-400 animate-pulse"></span>
            )}
          </button>
          <button
            onClick={() => setActiveTab('advanced')}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === 'advanced'
                ? 'border-purple-500 text-purple-400'
                : 'border-transparent text-zinc-400 hover:text-zinc-200'
            }`}
          >
            <Terminal className="w-4 h-4" />
            <span>Command Preview</span>
          </button>
        </div>

        {/* Tab Contents */}
        <div className="p-6 overflow-y-auto flex-1 space-y-6">
          {activeTab === 'general' && (
            <div className="space-y-4">
              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-zinc-400 mb-1.5">
                  Launch Arguments (Steam style)
                </label>
                <input
                  type="text"
                  value={settings.launch_arguments}
                  onChange={(e) => setSettings({ ...settings, launch_arguments: e.target.value })}
                  placeholder="-novid -fullscreen %command%"
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-lg px-3.5 py-2 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-purple-500 font-mono"
                />
                <p className="text-xs text-zinc-500 mt-1">Pass custom flags directly to the executable or proton runner.</p>
              </div>

              {/* Toggles Grid */}
              <div className="grid grid-cols-2 gap-4 pt-2">
                <div className="flex items-center justify-between p-3.5 rounded-xl bg-zinc-950/60 border border-zinc-800">
                  <div>
                    <h4 className="text-sm font-medium text-zinc-200">Feral GameMode</h4>
                    <p className="text-xs text-zinc-400">Request CPU/GPU performance mode</p>
                  </div>
                  <input
                    type="checkbox"
                    checked={settings.use_gamemode}
                    onChange={(e) => setSettings({ ...settings, use_gamemode: e.target.checked })}
                    className="w-5 h-5 accent-purple-600 rounded cursor-pointer"
                  />
                </div>

                <div className="flex items-center justify-between p-3.5 rounded-xl bg-zinc-950/60 border border-zinc-800">
                  <div>
                    <h4 className="text-sm font-medium text-zinc-200">MangoHud Overlay</h4>
                    <p className="text-xs text-zinc-400">FPS, temperatures, and frametime graph</p>
                  </div>
                  <input
                    type="checkbox"
                    checked={settings.use_mangohud}
                    onChange={(e) => setSettings({ ...settings, use_mangohud: e.target.checked })}
                    className="w-5 h-5 accent-purple-600 rounded cursor-pointer"
                  />
                </div>

                <div className="flex items-center justify-between p-3.5 rounded-xl bg-zinc-950/60 border border-zinc-800">
                  <div>
                    <h4 className="text-sm font-medium text-zinc-200">Gamescope Micro-Compositor</h4>
                    <p className="text-xs text-zinc-400">Isolated Wayland session with FSR / scaling</p>
                  </div>
                  <input
                    type="checkbox"
                    checked={settings.use_gamescope}
                    onChange={(e) => setSettings({ ...settings, use_gamescope: e.target.checked })}
                    className="w-5 h-5 accent-purple-600 rounded cursor-pointer"
                  />
                </div>

                <div className="flex items-center justify-between p-3.5 rounded-xl bg-zinc-950/60 border border-zinc-800">
                  <div>
                    <h4 className="text-sm font-medium text-zinc-200">Anti-Cheat Runtimes</h4>
                    <p className="text-xs text-zinc-400">Bind Proton EAC & BattlEye bridges</p>
                  </div>
                  <input
                    type="checkbox"
                    checked={settings.eac_runtime}
                    onChange={(e) => setSettings({ ...settings, eac_runtime: e.target.checked, battleye_runtime: e.target.checked })}
                    className="w-5 h-5 accent-purple-600 rounded cursor-pointer"
                  />
                </div>
              </div>

              {settings.use_gamescope && (
                <div className="p-4 rounded-xl bg-zinc-950 border border-zinc-800 space-y-3 animate-in fade-in">
                  <h4 className="text-xs font-semibold uppercase tracking-wider text-purple-400">Gamescope Configuration</h4>
                  <div className="grid grid-cols-3 gap-3">
                    <div>
                      <label className="text-xs text-zinc-400">Width (-w)</label>
                      <input
                        type="number"
                        value={settings.gamescope_w || ''}
                        onChange={(e) => setSettings({ ...settings, gamescope_w: parseInt(e.target.value) || 0 })}
                        placeholder="1920"
                        className="w-full mt-1 bg-zinc-900 border border-zinc-700 rounded px-2.5 py-1.5 text-sm text-zinc-100 font-mono"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-zinc-400">Height (-h)</label>
                      <input
                        type="number"
                        value={settings.gamescope_h || ''}
                        onChange={(e) => setSettings({ ...settings, gamescope_h: parseInt(e.target.value) || 0 })}
                        placeholder="1080"
                        className="w-full mt-1 bg-zinc-900 border border-zinc-700 rounded px-2.5 py-1.5 text-sm text-zinc-100 font-mono"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-zinc-400">Framerate (-r)</label>
                      <input
                        type="number"
                        value={settings.gamescope_r || ''}
                        onChange={(e) => setSettings({ ...settings, gamescope_r: parseInt(e.target.value) || 0 })}
                        placeholder="144"
                        className="w-full mt-1 bg-zinc-900 border border-zinc-700 rounded px-2.5 py-1.5 text-sm text-zinc-100 font-mono"
                      />
                    </div>
                  </div>
                </div>
              )}
            </div>
          )}

          {activeTab === 'runner' && (
            <div className="space-y-4">
              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-zinc-400 mb-1.5">
                  Compatibility Tool / Runner
                </label>
                <select
                  value={settings.runner_id}
                  onChange={(e) => setSettings({ ...settings, runner_id: e.target.value })}
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-lg px-3.5 py-2.5 text-sm text-zinc-100 focus:outline-none focus:border-purple-500"
                >
                  <option value="auto">Automatic (System / Steam Default)</option>
                  {protonVersions.map((pv) => (
                    <option key={pv.id} value={pv.id}>
                      {pv.name} {pv.is_custom ? '★' : ''} ({pv.path})
                    </option>
                  ))}
                </select>
                <p className="text-xs text-zinc-500 mt-1">
                  Discovered from Steam, compatibilitytools.d (GE-Proton), and Lutris Wine directories.
                </p>
              </div>

              <div>
                <label className="block text-xs font-semibold uppercase tracking-wider text-zinc-400 mb-1.5">
                  Isolated Wine Prefix (Optional)
                </label>
                <input
                  type="text"
                  value={settings.wine_prefix}
                  onChange={(e) => setSettings({ ...settings, wine_prefix: e.target.value })}
                  placeholder={`~/.local/share/hydra/prefixes/${game.id}`}
                  className="w-full bg-zinc-950 border border-zinc-800 rounded-lg px-3.5 py-2 text-sm text-zinc-100 placeholder-zinc-600 focus:outline-none focus:border-purple-500 font-mono"
                />
                <p className="text-xs text-zinc-500 mt-1">
                  Leave blank to use Hydra's automatic prefix isolation sandbox.
                </p>
              </div>
            </div>
          )}

          {activeTab === 'fixes' && (
            <div className="space-y-4">
              {game.fix_info ? (
                <div className="p-4 rounded-xl bg-zinc-950/80 border border-emerald-500/30 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="text-xs uppercase font-bold px-2 py-0.5 rounded bg-emerald-500/20 text-emerald-400 border border-emerald-500/40">
                        {game.fix_info.source} Fix Database
                      </span>
                      <span className="text-xs font-semibold text-zinc-300">
                        Tier: <span className="uppercase text-emerald-400">{game.fix_info.tier}</span>
                      </span>
                    </div>
                  </div>

                  <p className="text-sm text-zinc-300">
                    {game.fix_info.description}
                  </p>

                  {game.fix_info.winetricks && game.fix_info.winetricks.length > 0 && (
                    <div className="pt-2">
                      <h5 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-1.5">
                        Required Winetricks Packages:
                      </h5>
                      <div className="flex items-center gap-2 flex-wrap">
                        {game.fix_info.winetricks.map((trick) => (
                          <span
                            key={trick}
                            className="text-xs font-mono px-2 py-1 rounded bg-zinc-800 border border-zinc-700 text-zinc-200"
                          >
                            {trick}
                          </span>
                        ))}
                        <button
                          onClick={() => applyFixWinetricks(game.fix_info?.winetricks || [])}
                          className="ml-auto text-xs px-3 py-1 rounded-md bg-emerald-600/80 hover:bg-emerald-500 text-white font-medium flex items-center gap-1.5 shadow"
                        >
                          <Zap className="w-3.5 h-3.5" />
                          <span>Apply Winetricks Verbs</span>
                        </button>
                      </div>
                    </div>
                  )}

                  {game.fix_info.launch_args && (
                    <div className="pt-2">
                      <h5 className="text-xs font-semibold text-zinc-400 uppercase tracking-wider mb-1">
                        Community Recommended Arguments:
                      </h5>
                      <div className="p-2 rounded bg-zinc-900 font-mono text-xs text-purple-300 border border-zinc-800">
                        {game.fix_info.launch_args}
                      </div>
                    </div>
                  )}
                </div>
              ) : (
                <div className="p-8 text-center text-zinc-500 space-y-2">
                  <Wrench className="w-8 h-8 mx-auto text-zinc-600" />
                  <p className="text-sm">No specific fixes required for this game.</p>
                  <p className="text-xs">Proton or native compatibility runs cleanly out of the box.</p>
                </div>
              )}
            </div>
          )}

          {activeTab === 'advanced' && (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <label className="text-xs font-semibold uppercase tracking-wider text-zinc-400">
                  Constructed Execution Pipeline
                </label>
                <button
                  onClick={handleCopyCommand}
                  className="flex items-center gap-1 text-xs text-purple-400 hover:text-purple-300"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-emerald-400" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>{copied ? 'Copied' : 'Copy Command'}</span>
                </button>
              </div>

              <div className="p-4 rounded-xl bg-black border border-zinc-800 font-mono text-xs text-emerald-400 break-all leading-relaxed shadow-inner">
                {cmdPreview || 'Building pipeline...'}
              </div>
              <p className="text-xs text-zinc-500">
                This command is constructed dynamically using your chosen wrappers (GameMode, Gamescope, MangoHud) and selected Proton binary.
              </p>
            </div>
          )}
        </div>

        {/* Footer Actions */}
        <div className="p-4 bg-zinc-950 border-t border-zinc-800/80 flex items-center justify-end gap-3">
          <button
            onClick={onClose}
            className="px-4 py-2 rounded-lg text-sm text-zinc-400 hover:text-zinc-200 transition-colors"
          >
            Cancel
          </button>
          <button
            onClick={() => {
              onSave({ ...game, settings });
              onClose();
            }}
            className="px-5 py-2 rounded-lg bg-zinc-800 hover:bg-zinc-700 text-sm font-medium text-white border border-zinc-700 transition-colors"
          >
            Save Configuration
          </button>
        </div>
      </div>
    </div>
  );
};
