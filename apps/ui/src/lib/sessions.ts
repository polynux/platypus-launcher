/** A session the backend still tracks: the game is starting, running or being stopped. */
export const isLiveState = (state: string): boolean => state === 'starting' || state === 'running' || state === 'stopping';
