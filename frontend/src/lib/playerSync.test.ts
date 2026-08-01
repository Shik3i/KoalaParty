import { describe, expect, it } from 'vitest';
import {
  PLAYER_STATE,
  normalizedDuration,
  shouldReanchorPlayback,
  stateChangeAction,
  timelineJump,
  type StateChangeInput,
} from './playerSync';

const { ENDED, PLAYING, PAUSED, BUFFERING } = PLAYER_STATE;

// A sensible baseline: player is ready with a video loaded, the local viewer can
// control playback, and nothing programmatic is currently echoing.
const base: StateChangeInput = {
  state: PLAYING,
  serverStatus: 'playing',
  guarded: false,
  ready: true,
  hasVideo: true,
  videoMatches: true,
  currentTime: 98,
  duration: 100,
  canControl: true,
};

describe('stateChangeAction', () => {
  it('advances the queue on a natural end of video', () => {
    expect(stateChangeAction({ ...base, state: ENDED })).toBe('ended');
  });

  it('advances a confirmed natural end even while guarded', () => {
    expect(stateChangeAction({ ...base, state: ENDED, guarded: true })).toBe('ended');
  });

  it('ignores false ENDED events from initialization, video replacement and failed embeds', () => {
    expect(stateChangeAction({ ...base, state: ENDED, ready: false })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: ENDED, hasVideo: false })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: ENDED, videoMatches: false })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: ENDED, currentTime: 0, duration: 0 })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: ENDED, currentTime: 12, duration: 100 })).toBe('ignore');
    expect(stateChangeAction({ ...base, state: ENDED, currentTime: Number.NaN })).toBe('ignore');
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

describe('playback anchors', () => {
  it('does not re-anchor for an extrapolated position in an unrelated room snapshot', () => {
    expect(shouldReanchorPlayback({ mediaId: 'video-a', revision: 7 }, { mediaId: 'video-a', revision: 7 })).toBe(
      false,
    );
  });

  it('re-anchors for a player command or media replacement', () => {
    expect(shouldReanchorPlayback({ mediaId: 'video-a', revision: 7 }, { mediaId: 'video-a', revision: 8 })).toBe(true);
    expect(shouldReanchorPlayback({ mediaId: 'video-a', revision: 7 }, { mediaId: 'video-b', revision: 7 })).toBe(true);
  });
});

describe('player duration compatibility', () => {
  it('keeps finite on-demand video durations', () => {
    expect(normalizedDuration(4_375.961)).toBe(4_375.961);
  });

  it.each([0, -1, Number.NaN, Number.POSITIVE_INFINITY, 121_601_512])(
    'treats %s as an unavailable or live-stream duration',
    (duration) => expect(normalizedDuration(duration)).toBe(0),
  );
});

describe('timelineJump', () => {
  it('does not mistake natural 4x playback for a seek', () => {
    expect(timelineJump(12, 10, 0.5, true, 4)).toBeCloseTo(0);
  });

  it('still detects a real seek at accelerated playback', () => {
    expect(timelineJump(18, 10, 0.5, true, 4)).toBeCloseTo(6);
  });
});
