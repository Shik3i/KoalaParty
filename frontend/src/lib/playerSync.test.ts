import { describe, expect, it } from 'vitest';
import {
  PLAYER_STATE,
  isCurrentVideoError,
  driftAction,
  nextSeekLead,
  isLocalTimelineJump,
  isStableTimelineState,
  isRetryablePlayerError,
  isUnboundedTimeline,
  isWrappedEndedPlayback,
  normalizedDuration,
  playerErrorMessage,
  shouldReanchorPlayback,
  shouldRecoverPlayback,
  stateChangeAction,
  timelineJump,
  timelineRecoveryJump,
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
    expect(stateChangeAction({ ...base, state: ENDED, currentTime: 0, duration: 1 })).toBe('ignore');
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

describe('playback recovery', () => {
  it.each([PLAYING, BUFFERING, ENDED])('does not restart state %s after a presentation transition', (state) => {
    expect(shouldRecoverPlayback('playing', true, state)).toBe(false);
  });

  it('recognizes an ended server clock that YouTube wrapped to the beginning after reload', () => {
    expect(isWrappedEndedPlayback('playing', 1.2, 100.4, 100)).toBe(true);
    expect(isWrappedEndedPlayback('playing', 3, 635, 635)).toBe(true);
  });

  it('does not confuse ordinary playback, a local seek or a paused room with a wrapped end', () => {
    expect(isWrappedEndedPlayback('playing', 98, 100, 100)).toBe(false);
    expect(isWrappedEndedPlayback('playing', 1, 30, 100)).toBe(false);
    expect(isWrappedEndedPlayback('paused', 1, 100, 100)).toBe(false);
  });

  it('recovers a stale ENDED state after the room has advanced to another media item', () => {
    expect(shouldRecoverPlayback('playing', true, ENDED, false)).toBe(true);
    expect(shouldRecoverPlayback('playing', true, ENDED, true)).toBe(false);
  });

  it('wakes a paused or cued iframe only while the room is still playing that media', () => {
    expect(shouldRecoverPlayback('playing', true, PAUSED)).toBe(true);
    expect(shouldRecoverPlayback('playing', true, 5)).toBe(true);
    expect(shouldRecoverPlayback('paused', true, PAUSED)).toBe(false);
    expect(shouldRecoverPlayback('playing', false, PAUSED)).toBe(false);
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
  it('detects a pseudo-duration live stream', () => {
    expect(isUnboundedTimeline(121_601_512, 4_854_202.6)).toBe(true);
  });
  it('detects an absolute live timeline when duration is unavailable', () => {
    expect(isUnboundedTimeline(0, 4_854_202.6)).toBe(true);
  });
  it('keeps ordinary VOD timelines bounded', () => {
    expect(isUnboundedTimeline(214, 42)).toBe(false);
  });
});

describe('timeline state stability', () => {
  it.each([PLAYING, PAUSED])('accepts stable state %s for scrub detection', (state) => {
    expect(isStableTimelineState(state)).toBe(true);
  });
  it.each([ENDED, BUFFERING, -1, 5])('rejects transient state %s for scrub detection', (state) => {
    expect(isStableTimelineState(state)).toBe(false);
  });
  it('does not broadcast a buffering reset as a seek', () => {
    expect(isLocalTimelineJump(BUFFERING, -2, 1.5)).toBe(false);
  });
  it('broadcasts a real backward scrub while playing', () => {
    expect(isLocalTimelineJump(PLAYING, -2, 1.5)).toBe(true);
  });
  it('detects a user scrub after YouTube buffered the seek', () => {
    const jump = timelineRecoveryJump(70, 10, 1, true, 1);
    expect(isLocalTimelineJump(PLAYING, jump, 1.5)).toBe(true);
  });
  it('does not mistake a stalled or naturally advancing buffer recovery for a seek', () => {
    expect(timelineRecoveryJump(10, 10, 4, true, 1)).toBe(0);
    expect(timelineRecoveryJump(12, 10, 4, true, 1)).toBe(0);
    expect(timelineRecoveryJump(14, 10, 4, true, 1)).toBe(0);
  });
  it('detects a backward scrub after buffering', () => {
    const jump = timelineRecoveryJump(10, 70, 1, true, 1);
    expect(isLocalTimelineJump(PLAYING, jump, 1.5)).toBe(true);
  });
});

describe('player error attribution', () => {
  it('accepts an active or not-yet-reported video and rejects a stale one', () => {
    expect(isCurrentVideoError('active', 'active')).toBe(true);
    expect(isCurrentVideoError('active', '')).toBe(true);
    expect(isCurrentVideoError('active', 'replacement')).toBe(false);
    expect(isCurrentVideoError(null, 'active')).toBe(false);
  });

  it.each([
    [2, 'YouTube rejected this video request.'],
    [5, 'YouTube could not play this video in the embedded player.'],
    [100, 'This YouTube video no longer exists.'],
    [101, 'This video does not allow embedded playback.'],
    [150, 'This video does not allow embedded playback.'],
    [153, 'YouTube could not verify the embedded player origin.'],
    [999, 'This video is unavailable or cannot be embedded.'],
  ])('maps YouTube error %s to a useful message', (code, message) => {
    expect(playerErrorMessage(code)).toBe(message);
  });

  it('only retries transient or unknown player failures', () => {
    expect(isRetryablePlayerError(5)).toBe(true);
    expect(isRetryablePlayerError(0)).toBe(true);
    expect(isRetryablePlayerError(2)).toBe(true);
    expect(isRetryablePlayerError(153)).toBe(false);
    expect(isRetryablePlayerError(150)).toBe(false);
    expect(isRetryablePlayerError(999)).toBe(true);
  });
});

describe('timelineJump', () => {
  it('does not mistake natural 4x playback for a seek', () => {
    expect(timelineJump(12, 10, 0.5, true, 4)).toBeCloseTo(0);
  });

  it('still detects a real seek at accelerated playback', () => {
    expect(timelineJump(18, 10, 0.5, true, 4)).toBeCloseTo(6);
  });
});

describe('tiered drift correction', () => {
  const base = { now: 100_000, softSince: null, lastSoftCorrection: 0 };
  it('corrects hard drift immediately and small paused drift precisely', () => {
    expect(driftAction({ ...base, drift: 2.1, playing: true })).toBe('correct');
    expect(driftAction({ ...base, drift: -0.5, playing: false })).toBe('correct');
    expect(driftAction({ ...base, drift: 0.2, playing: false })).toBe('none');
  });
  it('only corrects sustained moderate drift while playing, with a cooldown', () => {
    expect(driftAction({ ...base, drift: 1.1, playing: true })).toBe('none');
    expect(driftAction({ ...base, drift: 1.1, playing: true, softSince: 99_000 })).toBe('none');
    expect(driftAction({ ...base, drift: 1.1, playing: true, softSince: 97_000 })).toBe('correct');
    expect(driftAction({ ...base, drift: 1.1, playing: true, softSince: 97_000, lastSoftCorrection: 95_000 })).toBe(
      'none',
    );
    expect(driftAction({ ...base, drift: 0.4, playing: true, softSince: 90_000 })).toBe('none');
  });
});

describe('adaptive seek lead', () => {
  it('learns to aim ahead of buffering and stays bounded', () => {
    expect(nextSeekLead(0, -0.6)).toBe(0.42);
    expect(nextSeekLead(0.42, -0.2)).toBe(0.56);
    expect(nextSeekLead(0.56, 0.3)).toBe(0.35);
    expect(nextSeekLead(1.4, -1)).toBe(1.5);
    expect(nextSeekLead(0.2, 0.9)).toBe(0);
    expect(nextSeekLead(0.5, 12)).toBe(0.5);
  });
});
