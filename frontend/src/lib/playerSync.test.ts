import { describe, expect, it } from 'vitest';
import { PLAYER_STATE, stateChangeAction, type StateChangeInput } from './playerSync';

const { ENDED, PLAYING, PAUSED, BUFFERING } = PLAYER_STATE;

// A sensible baseline: player is ready with a video loaded, the local viewer can
// control playback, and nothing programmatic is currently echoing.
const base: StateChangeInput = {
  state: PLAYING,
  serverStatus: 'playing',
  guarded: false,
  ready: true,
  hasVideo: true,
  canControl: true,
};

describe('stateChangeAction', () => {
  it('advances the queue on a natural end of video', () => {
    expect(stateChangeAction({ ...base, state: ENDED })).toBe('ended');
  });

  it('advances on end of video even while guarded or before ready', () => {
    expect(stateChangeAction({ ...base, state: ENDED, guarded: true })).toBe('ended');
    expect(stateChangeAction({ ...base, state: ENDED, ready: false, hasVideo: false })).toBe('ended');
  });

  it('ignores state changes before the player is ready or with no video', () => {
    expect(stateChangeAction({ ...base, state: PAUSED, ready: false })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: PLAYING, hasVideo: false, serverStatus: 'paused' })).toBe('ignore');
  });

  // The core regression: a browser that blocks autoplay reports the freshly loaded,
  // playing video as PAUSED. While guarded this is our own echo and must NOT be
  // relayed, otherwise a controlling viewer would pause the video for the whole room.
  it('never forwards a blocked-autoplay pause while guarded', () => {
    expect(stateChangeAction({ ...base, state: PAUSED, serverStatus: 'playing', guarded: true })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: PLAYING, serverStatus: 'paused', guarded: true })).toBe('ignore');
  });

  it('forwards a genuine local pause when not guarded', () => {
    expect(stateChangeAction({ ...base, state: PAUSED, serverStatus: 'playing' })).toBe('emit-pause');
  });

  it('forwards a genuine local play when not guarded', () => {
    expect(stateChangeAction({ ...base, state: PLAYING, serverStatus: 'paused' })).toBe('emit-play');
  });

  it('snaps a viewer without control back to the authoritative state', () => {
    // They played a room the server has paused -> force them back to paused.
    expect(stateChangeAction({ ...base, state: PLAYING, serverStatus: 'paused', canControl: false })).toBe(
      'snap-pause',
    );
    // They paused a room the server is playing -> force them back to playing.
    expect(stateChangeAction({ ...base, state: PAUSED, serverStatus: 'playing', canControl: false })).toBe('snap-play');
  });

  it('ignores redundant changes that already match the server', () => {
    expect(stateChangeAction({ ...base, state: PLAYING, serverStatus: 'playing' })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: PAUSED, serverStatus: 'paused' })).toBe('ignore');
  });

  it('ignores transient buffering', () => {
    expect(stateChangeAction({ ...base, state: BUFFERING, serverStatus: 'playing' })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: BUFFERING, serverStatus: 'paused' })).toBe('ignore');
  });
});
