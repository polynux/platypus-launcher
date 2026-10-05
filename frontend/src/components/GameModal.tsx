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

export const GameModal: React.FC<GameModalProps> = (props) => {
  if (!props.game) return null;
  return <GameModalInner key={props.game.id} {...props} game={props.game} />;
};

const GameModalInner: React.FC<GameModalProps & { game: Game }> = ({
  game,
  protonVersions,
  onClose,
  onSave,
  onLaunch,
}) => {

  const [activeTab, setActiveTab] = useState<'general' | 'runner' | 'fixes' | 'advanced'>('general');
  const [settings, setSettings] = useState(game.settings);
  const [cmdPreview, setCmdPreview] = useState<string>('');
  const [copied, setCopied] = useState(false);

  // Update preview command when settings change
  useEffect(() => {
    const updatedGame = { ...game, settings };
    Effect.runPromise(LauncherApi.previewCommand(updatedGame)).then((res) => {
      setCmdPreview(res);
    }).catch((err: unknown) => {
      setCmdPreview(`# preview unavailable: ${err instanceof Error ? err.message : String(err)}`);
    });
  }, [settings, game]);

  const handleCopyCommand = () => {
    navigator.clipboard.writeText(cmdPreview).catch(() => {});
    setCopied(true);
    setTimeout(() => setCopied(false), 2000);
  };

  const applyFixWinetricks = (tricks: string[]) => {
    alert(`Applying winetricks verbs to prefix: ${tricks.join(', ')}`);
  };

  return (
    <div className="fixed inset-0 z-50 flex items-center justify-center bg-black/75 p-6 animate-in fade-in duration-150">
      <div
        className="bg-[#16181b] border border-[#2e3238] w-full max-w-4xl max-h-[90vh] flex flex-col overflow-hidden"
        style={{ boxShadow: '0 24px 60px rgba(0,0,0,.55), 0 8px 20px rgba(0,0,0,.4)' }}
      >
        {/* Header with Banner Image */}
        <div className="relative h-40 bg-[#101214] flex-shrink-0">
          <img
            src={game.banner_url || game.cover_url}
            alt=""
            className="w-full h-full object-cover opacity-45"
          />
          <div className="absolute inset-0" style={{ background: 'linear-gradient(to top, #16181b 4%, rgba(22,24,27,.55) 55%, rgba(22,24,27,.25))' }} />

          <button
            onClick={onClose}
            className="absolute top-3 right-3 w-7 h-7 bg-[#1d2024]/90 hover:bg-[#232327] text-[#9aa0a6] hover:text-[#e8e6e3] border border-[#3a3f45] flex items-center justify-center transition-colors"
          >
            <X className="w-3.5 h-3.5" />
          </button>

          <div className="absolute bottom-3 left-5 right-5 flex items-end justify-between">
            <div>
              <span
                className="font-mono text-[9px] uppercase tracking-[0.14em] px-1.5 py-0.5 bg-[#101214]/80 text-[#ffb000] border border-[#ffb000]/40"
                style={{ fontFamily: 'Hack, monospace' }}
              >
                {game.store}
              </span>
              <h2 className="text-[22px] font-bold text-[#f2f1ee] mt-1 leading-tight" style={{ fontFamily: '"Source Sans 3", system-ui, sans-serif' }}>
                {game.title}
              </h2>
            </div>
            <button
              onClick={() => onLaunch({ ...game, settings })}
              className="flex items-center gap-2 px-4 py-2 bg-[#ffb000] hover:bg-[#ffc23d] text-[#101214] text-[13px] font-semibold transition-colors active:translate-y-px"
              style={{ fontFamily: 'Hack, monospace' }}
            >
              <Play className="w-3.5 h-3.5 fill-current" />
              <span>LAUNCH</span>
            </button>
          </div>
        </div>

        {/* Tab Navigation */}
        <div className="flex border-b border-[#2a2d31] px-4 bg-[#181a1e]">
          <button
            onClick={() => setActiveTab('general')}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === 'general' ? 'border-[#ffb000] text-[#ffb000]' : 'border-transparent text-[#9aa0a6] hover:text-[#e8e6e3]'
            }`}
          >
            <Sliders className="w-4 h-4" />
            <span>Launch Options</span>
          </button>
          <button
            onClick={() => setActiveTab('runner')}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === 'runner' ? 'border-[#ffb000] text-[#ffb000]' : 'border-transparent text-[#9aa0a6] hover:text-[#e8e6e3]'
            }`}
          >
            <Cpu className="w-4 h-4" />
            <span>Proton & Wine</span>
          </button>
          <button
            onClick={() => setActiveTab('fixes')}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === 'fixes' ? 'border-[#ffb000] text-[#ffb000]' : 'border-transparent text-[#9aa0a6] hover:text-[#e8e6e3]'
            }`}
          >
            <Wrench className="w-4 h-4 text-emerald-400" />
            <span>Fixes & Tweaks</span>
            {game.fix_info && (
              <span className="w-1.5 h-1.5 bg-[#ffb000]"></span>
            )}
          </button>
          <button
            onClick={() => setActiveTab('advanced')}
            className={`px-4 py-3 text-sm font-medium border-b-2 transition-colors flex items-center gap-2 ${
              activeTab === 'advanced' ? 'border-[#ffb000] text-[#ffb000]' : 'border-transparent text-[#9aa0a6] hover:text-[#e8e6e3]'
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
                <label className="block font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[#9aa0a6] mb-1.5">
                  Launch Arguments (Steam style)
                </label>
                <input
                  type="text"
                  value={settings.launch_arguments}
                  onChange={(e) => setSettings({ ...settings, launch_arguments: e.target.value })}
                  placeholder="-novid -fullscreen %command%"
                  className="w-full bg-[#101214] border border-[#2e3238] rounded-[3px] px-3 py-2 text-[13px] text-[#e8e6e3] placeholder-[#5a6066] focus:outline-none focus:border-[#ffb000]/70 font-mono"
                />
                <p className="text-xs text-[#767c82] mt-1">Pass custom flags directly to the executable or proton runner.</p>
              </div>

              {/* Toggles Grid */}
              <div className="grid grid-cols-2 gap-4 pt-2">
                <div className="flex items-center justify-between p-3.5 bg-[#101214] border border-[#2e3238]">
                  <div>
                    <h4 className="text-[13px] font-medium text-[#e8e6e3]">Feral GameMode</h4>
                    <p className="text-xs text-[#9aa0a6]">Request CPU/GPU performance mode</p>
                  </div>
                  <input
                    type="checkbox"
                    checked={settings.use_gamemode}
                    onChange={(e) => setSettings({ ...settings, use_gamemode: e.target.checked })}
                    className="w-5 h-5 accent-[#ffb000] rounded cursor-pointer"
                  />
                </div>

                <div className="flex items-center justify-between p-3.5 bg-[#101214] border border-[#2e3238]">
                  <div>
                    <h4 className="text-[13px] font-medium text-[#e8e6e3]">MangoHud Overlay</h4>
                    <p className="text-xs text-[#9aa0a6]">FPS, temperatures, and frametime graph</p>
                  </div>
                  <input
                    type="checkbox"
                    checked={settings.use_mangohud}
                    onChange={(e) => setSettings({ ...settings, use_mangohud: e.target.checked })}
                    className="w-5 h-5 accent-[#ffb000] rounded cursor-pointer"
                  />
                </div>

                <div className="flex items-center justify-between p-3.5 bg-[#101214] border border-[#2e3238]">
                  <div>
                    <h4 className="text-[13px] font-medium text-[#e8e6e3]">Gamescope Micro-Compositor</h4>
                    <p className="text-xs text-[#9aa0a6]">Isolated Wayland session with FSR / scaling</p>
                  </div>
                  <input
                    type="checkbox"
                    checked={settings.use_gamescope}
                    onChange={(e) => setSettings({ ...settings, use_gamescope: e.target.checked })}
                    className="w-5 h-5 accent-[#ffb000] rounded cursor-pointer"
                  />
                </div>

                <div className="flex items-center justify-between p-3.5 bg-[#101214] border border-[#2e3238]">
                  <div>
                    <h4 className="text-[13px] font-medium text-[#e8e6e3]">Anti-Cheat Runtimes</h4>
                    <p className="text-xs text-[#9aa0a6]">Bind Proton EAC &amp; BattlEye bridges</p>
                  </div>
                  <div className="flex items-center gap-4">
                    <label className="flex items-center gap-1.5 font-mono text-[10px] text-[#c9cdd1] cursor-pointer">
                      <input
                        type="checkbox"
                        checked={settings.eac_runtime}
                        onChange={(e) => setSettings({ ...settings, eac_runtime: e.target.checked })}
                        className="w-4 h-4 accent-[#ffb000] rounded cursor-pointer"
                      />
                      EAC
                    </label>
                    <label className="flex items-center gap-1.5 font-mono text-[10px] text-[#c9cdd1] cursor-pointer">
                      <input
                        type="checkbox"
                        checked={settings.battleye_runtime}
                        onChange={(e) => setSettings({ ...settings, battleye_runtime: e.target.checked })}
                        className="w-4 h-4 accent-[#ffb000] rounded cursor-pointer"
                      />
                      BattlEye
                    </label>
                  </div>
                </div>
              </div>

              {settings.use_gamescope && (
                <div className="p-4 bg-[#101214] border border-[#2e3238] space-y-3 animate-in fade-in">
                  <h4 className="font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[#ffb000]">Gamescope Configuration</h4>
                  <div className="grid grid-cols-3 gap-3">
                    <div>
                      <label className="text-xs text-[#9aa0a6]">Width (-w)</label>
                      <input
                        type="number"
                        value={settings.gamescope_w || ''}
                        onChange={(e) => setSettings({ ...settings, gamescope_w: parseInt(e.target.value) || 0 })}
                        placeholder="1920"
                        className="w-full mt-1 bg-[#16181b] border border-[#2e3238] rounded-[3px] px-2 py-1.5 text-[13px] text-[#e8e6e3] font-mono"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-[#9aa0a6]">Height (-h)</label>
                      <input
                        type="number"
                        value={settings.gamescope_h || ''}
                        onChange={(e) => setSettings({ ...settings, gamescope_h: parseInt(e.target.value) || 0 })}
                        placeholder="1080"
                        className="w-full mt-1 bg-[#16181b] border border-[#2e3238] rounded-[3px] px-2 py-1.5 text-[13px] text-[#e8e6e3] font-mono"
                      />
                    </div>
                    <div>
                      <label className="text-xs text-[#9aa0a6]">Framerate (-r)</label>
                      <input
                        type="number"
                        value={settings.gamescope_r || ''}
                        onChange={(e) => setSettings({ ...settings, gamescope_r: parseInt(e.target.value) || 0 })}
                        placeholder="144"
                        className="w-full mt-1 bg-[#16181b] border border-[#2e3238] rounded-[3px] px-2 py-1.5 text-[13px] text-[#e8e6e3] font-mono"
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
                <label className="block font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[#9aa0a6] mb-1.5">
                  Compatibility Tool / Runner
                </label>
                <select
                  value={settings.runner_id}
                  onChange={(e) => setSettings({ ...settings, runner_id: e.target.value })}
                  className="w-full bg-[#101214] border border-[#2e3238] rounded-[3px] px-3 py-2 text-[13px] text-[#e8e6e3] focus:outline-none focus:border-[#ffb000]/70"
                >
                  <option value="auto">Automatic (System / Steam Default)</option>
                  {protonVersions.map((pv) => (
                    <option key={pv.id} value={pv.id}>
                      {pv.name} {pv.is_custom ? '★' : ''} ({pv.path})
                    </option>
                  ))}
                </select>
                <p className="text-xs text-[#767c82] mt-1">
                  Discovered from Steam, compatibilitytools.d (GE-Proton), and Lutris Wine directories.
                </p>
              </div>

              <div>
                <label className="block font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[#9aa0a6] mb-1.5">
                  Isolated Wine Prefix (Optional)
                </label>
                <input
                  type="text"
                  value={settings.wine_prefix}
                  onChange={(e) => setSettings({ ...settings, wine_prefix: e.target.value })}
                  placeholder={`~/.local/share/hydra/prefixes/${game.id}`}
                  className="w-full bg-[#101214] border border-[#2e3238] rounded-[3px] px-3 py-2 text-[13px] text-[#e8e6e3] placeholder-[#5a6066] focus:outline-none focus:border-[#ffb000]/70 font-mono"
                />
                <p className="text-xs text-[#767c82] mt-1">
                  Leave blank to use Hydra's automatic prefix isolation sandbox.
                </p>
              </div>
            </div>
          )}

          {activeTab === 'fixes' && (
            <div className="space-y-4">
              {game.fix_info ? (
                <div className="p-4 bg-[#101214] border border-[#7dd48a]/30 space-y-3">
                  <div className="flex items-center justify-between">
                    <div className="flex items-center gap-2">
                      <span className="font-mono text-[9px] uppercase tracking-[0.12em] px-1.5 py-0.5 bg-[#7dd48a]/10 text-[#7dd48a] border border-[#7dd48a]/40">
                        {game.fix_info.source} Fix Database
                      </span>
                      <span className="text-xs font-semibold text-[#c9cdd1]">
                        Tier: <span className="uppercase text-[#7dd48a]">{game.fix_info.tier}</span>
                      </span>
                    </div>
                  </div>

                  <p className="text-[13px] text-[#c9cdd1]">
                    {game.fix_info.description}
                  </p>

                  {game.fix_info.winetricks && game.fix_info.winetricks.length > 0 && (
                    <div className="pt-2">
                      <h5 className="font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[#9aa0a6] mb-1.5">
                        Required Winetricks Packages:
                      </h5>
                      <div className="flex items-center gap-2 flex-wrap">
                        {game.fix_info.winetricks.map((trick) => (
                          <span
                            key={trick}
                            className="font-mono text-[11px] px-1.5 py-0.5 bg-[#1d2024] border border-[#3a3f45] text-[#e8e6e3] rounded-[3px]"
                          >
                            {trick}
                          </span>
                        ))}
                        <button
                          onClick={() => applyFixWinetricks(game.fix_info?.winetricks || [])}
                          className="ml-auto font-mono text-[10px] px-2.5 py-1 bg-[#1d2024] hover:border-[#ffb000] border border-[#3a3f45] text-[#ffb000] font-medium flex items-center gap-1.5 transition-colors rounded-[3px]"
                        >
                          <Zap className="w-3.5 h-3.5" />
                          <span>Apply Winetricks Verbs</span>
                        </button>
                      </div>
                    </div>
                  )}

                  {game.fix_info.launch_args && (
                    <div className="pt-2">
                      <h5 className="font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[#9aa0a6] mb-1">
                        Community Recommended Arguments:
                      </h5>
                      <div className="p-2 bg-[#16181b] font-mono text-[11px] text-[#ffdf9e] border border-[#2e3238] rounded-[3px]">
                        {game.fix_info.launch_args}
                      </div>
                    </div>
                  )}
                </div>
              ) : (
                <div className="p-8 text-center text-[#767c82] space-y-2 border border-[#2e3238] bg-[#101214]">
                  <Wrench className="w-7 h-7 mx-auto text-[#4a4f55]" strokeWidth={1.5} />
                  <p className="font-mono text-[11px] uppercase tracking-[0.12em]" style={{ fontFamily: 'Hack, monospace' }}>no fixes on file</p>
                  <p className="text-xs text-[#9aa0a6]">Proton or native compatibility should run cleanly out of the box.</p>
                </div>
              )}
            </div>
          )}

          {activeTab === 'advanced' && (
            <div className="space-y-3">
              <div className="flex items-center justify-between">
                <label className="font-mono text-[10px] font-semibold uppercase tracking-[0.12em] text-[#9aa0a6]">
                  Constructed Execution Pipeline
                </label>
                <button
                  onClick={handleCopyCommand}
                  className="flex items-center gap-1 font-mono text-[10px] text-[#ffb000] hover:text-[#ffc23d]"
                >
                  {copied ? <Check className="w-3.5 h-3.5 text-[#7dd48a]" /> : <Copy className="w-3.5 h-3.5" />}
                  <span>{copied ? 'COPIED' : 'COPY CMD'}</span>
                </button>
              </div>

              <div className="p-4 bg-[#0c0e10] border border-[#2e3238] font-mono text-[11px] text-[#ffdf9e] break-all leading-relaxed">
                {cmdPreview || 'building pipeline…'}
              </div>
              <p className="text-xs text-[#767c82]">
                This command is constructed dynamically using your chosen wrappers (GameMode, Gamescope, MangoHud) and selected Proton binary.
              </p>
            </div>
          )}
        </div>

        {/* Footer Actions */}
        <div className="px-4 py-3 bg-[#131519] border-t border-[#2a2d31] flex items-center justify-end gap-2.5">
          <button
            onClick={onClose}
            className="px-3.5 py-1.5 font-mono text-[11px] text-[#9aa0a6] hover:text-[#e8e6e3] transition-colors"
            style={{ fontFamily: 'Hack, monospace' }}
          >
            CANCEL
          </button>
          <button
            onClick={() => {
              onSave({ ...game, settings });
              onClose();
            }}
            className="px-4 py-1.5 bg-[#1d2024] hover:border-[#ffb000]/70 hover:text-[#ffb000] font-mono text-[11px] text-[#e8e6e3] border border-[#3a3f45] transition-colors rounded-[3px]"
            style={{ fontFamily: 'Hack, monospace' }}
          >
            SAVE CONFIG
          </button>
        </div>
      </div>
    </div>
  );
};
