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
  videoMatches: boolean; // the iframe still reports the media we believe is loaded
  currentTime: number; // the iframe's current media position
  duration: number; // the iframe's duration; unavailable/failed embeds report 0
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
// real pause and stop the video for everyone in the room. ENDED is handled more
// strictly: YouTube can emit it while replacing a video or failing an embed, so it
// only advances when the current iframe media is verifiably at its natural end.
export function stateChangeAction(i: StateChangeInput): StateChangeAction {
  if (!i.ready || !i.hasVideo) return 'ignore';
  if (i.state === PLAYER_STATE.ENDED) {
    const finiteTimeline = Number.isFinite(i.currentTime) && Number.isFinite(i.duration) && i.duration > 0;
    const endTolerance = Math.max(2, Math.min(5, i.duration * 0.01));
    return i.videoMatches && finiteTimeline && i.currentTime >= i.duration - endTolerance ? 'ended' : 'ignore';
  }
  if (i.guarded) return 'ignore';
  if (i.state === PLAYER_STATE.PLAYING && i.serverStatus !== 'playing') {
    return i.canControl ? 'emit-play' : 'snap-pause';
  }
  if (i.state === PLAYER_STATE.PAUSED && i.serverStatus === 'playing') {
    return i.canControl ? 'emit-pause' : 'snap-play';
  }
  return 'ignore';
}

export interface PlaybackAnchorKey {
  mediaId: string;
  revision: number;
}

// Room snapshots continuously extrapolate `playback.position` while playing. The
// playback revision is the stable signal that an actual player command happened;
// queue, title, participant, and activity snapshots must not restart the player.
export function shouldReanchorPlayback(current: PlaybackAnchorKey, next: PlaybackAnchorKey): boolean {
  return current.mediaId !== next.mediaId || current.revision !== next.revision;
}

// The iframe API reports epoch-like pseudo durations for some continuous live
// streams. KoalaParty positions are bounded to seven days server-side, so anything
// beyond that is not a useful seekable duration and must not drive progress/end UI.
export function normalizedDuration(duration: number): number {
  return Number.isFinite(duration) && duration > 0 && duration <= 604_800 ? duration : 0;
}

// The IFrame API may report an empty video ID during its first error callback,
// but an ID belonging to another loaded item is a stale callback from a prior
// request and must not cover the active player.
export function isCurrentVideoError(expectedVideoId: string | null, reportedVideoId: string): boolean {
  return !!expectedVideoId && (!reportedVideoId || reportedVideoId === expectedVideoId);
}

export function playerErrorMessage(code: number): string {
  switch (code) {
    case 2:
      return 'YouTube rejected this video request.';
    case 5:
      return 'YouTube could not play this video in the embedded player.';
    case 100:
      return 'This YouTube video no longer exists.';
    case 101:
    case 150:
      return 'This video does not allow embedded playback.';
    case 153:
      return 'YouTube could not verify the embedded player origin.';
    default:
      return 'This video is unavailable or cannot be embedded.';
  }
}

export function isRetryablePlayerError(code: number): boolean {
  return code === 0 || code === 5 || ![2, 100, 101, 150, 153].includes(code);
}

export function timelineJump(
  currentTime: number,
  previousTime: number,
  elapsedSeconds: number,
  playing: boolean,
  playbackRate: number,
): number {
  const naturalAdvance = playing ? Math.max(0, elapsedSeconds) * Math.max(0, playbackRate || 1) : 0;
  return currentTime - previousTime - naturalAdvance;
}
