// The YouTube IFrame player states we react to.
export const PLAYER_STATE = {
  ENDED: 0,
  PLAYING: 1,
  PAUSED: 2,
  BUFFERING: 3,
} as const;

export type StateChangeAction =
  | 'ended' // natural end of video: advance the queue
  | 'ignore' // nothing to do (or a self-induced echo inside the guard window)
  | 'emit-play' // local viewer started playback: tell the server
  | 'emit-pause' // local viewer paused playback: tell the server
  | 'snap-play' // viewer without control paused a playing room: force play back
  | 'snap-pause'; // viewer without control played a paused room: force pause back

export interface StateChangeInput {
  state: number;
  serverStatus: string; // the authoritative status: 'playing' or 'paused'
  guarded: boolean; // true while a recent programmatic action still echoes back
  ready: boolean; // the player has fired onReady
  hasVideo: boolean; // a video is currently loaded (lastVideo set)
  canControl: boolean; // this viewer may drive playback for the room
}

// Pure decision for what a raw YouTube state change means. Extracted from the
// component so the guard / phantom-gesture logic is unit-testable in isolation.
//
// The central rule: any state change that happens while `guarded` is true is the
// echo of our OWN programmatic control — loading a video, the muted-autoplay
// fallback, a correcting seek, or a requested play — and must never be relayed to
// the server. In particular a browser that blocks autoplay reports the video as
// PAUSED; without the guard a passive-but-controlling viewer would forward that as a
// real pause and stop the video for everyone in the room. A true end-of-video is the
// one exception: it must always advance the queue, even inside the guard window.
export function stateChangeAction(i: StateChangeInput): StateChangeAction {
  if (i.state === PLAYER_STATE.ENDED) return 'ended';
  if (!i.ready || !i.hasVideo) return 'ignore';
  if (i.guarded) return 'ignore';
  if (i.state === PLAYER_STATE.PLAYING && i.serverStatus !== 'playing') {
    return i.canControl ? 'emit-play' : 'snap-pause';
  }
  if (i.state === PLAYER_STATE.PAUSED && i.serverStatus === 'playing') {
    return i.canControl ? 'emit-pause' : 'snap-play';
  }
  return 'ignore';
}
