import { describe, it, expect } from 'vitest';
import { PAUSED_CLASS, pauseAllVideos, shouldPause } from './pauseWhileHidden';

describe('shouldPause (issue #49)', () => {
  it('pauses whenever the page is hidden, game or not', () => {
    expect(shouldPause({ hidden: true, blurred: false, gameRunning: false })).toBe(true);
    expect(shouldPause({ hidden: true, blurred: true, gameRunning: true })).toBe(true);
  });

  it('pauses when blurred only while a game runs', () => {
    expect(shouldPause({ hidden: false, blurred: true, gameRunning: true })).toBe(true);
    expect(shouldPause({ hidden: false, blurred: true, gameRunning: false })).toBe(false);
  });

  it('never pauses a focused, visible window', () => {
    expect(shouldPause({ hidden: false, blurred: false, gameRunning: false })).toBe(false);
    expect(shouldPause({ hidden: false, blurred: false, gameRunning: true })).toBe(false);
  });
});

describe('pauseAllVideos', () => {
  it('pauses every video element in the page', () => {
    document.body.innerHTML = '<video id="a"></video><video id="b"></video>';
    const paused: string[] = [];
    for (const v of document.querySelectorAll('video')) {
      (v as HTMLVideoElement).pause = () => paused.push((v as HTMLVideoElement).id);
    }
    pauseAllVideos();
    expect(paused.sort()).toEqual(['a', 'b']);
    document.body.innerHTML = '';
  });

  it('exposes the paused class name the stylesheet stops animations with', () => {
    expect(PAUSED_CLASS).toBe('platypus-paused');
    expect(document.documentElement.className).not.toContain('platypus-paused');
  });
});
