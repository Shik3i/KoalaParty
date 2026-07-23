export type Role = 'owner' | 'admin' | 'member';
export interface Media {
  id: string;
  providerId: string;
  title: string;
  thumbnail: string;
}
export interface QueueItem {
  id: string;
  position: number;
  media: Media;
  votes: number;
  voted: boolean;
}
export interface Member {
  identityId: string;
  displayName: string;
  role: Role;
  active: boolean;
  accountLinked: boolean;
  permissions: Record<string, boolean>;
}
export interface Activity {
  id: string;
  actorId?: string;
  actorName?: string;
  type: string;
  payload: Record<string, unknown>;
  createdAt: string;
}
export interface SponsorSegment {
  start: number;
  end: number;
  category: string;
}
// SponsorBlock categories KoalaParty acts on by default (skips). The server fetches
// a wider set; the room only skips these unless changed.
export const SKIPPED_SPONSOR_CATEGORIES = ['sponsor', 'selfpromo', 'intro', 'outro', 'interaction'];
export const SPONSOR_CATEGORY_LABELS: Record<string, string> = {
  sponsor: 'Sponsor',
  selfpromo: 'Self-promotion',
  intro: 'Intro',
  outro: 'Outro',
  interaction: 'Interaction reminder',
  preview: 'Preview',
  music_offtopic: 'Non-music',
};
export interface Snapshot {
  id: string;
  label: string;
  visibility: string;
  me: string;
  members: Member[];
  queue: QueueItem[];
  history: Media[];
  queueLoop: boolean;
  sponsorBlock: boolean;
  playback: {
    media: Media | null;
    status: string;
    position: number;
    rate: number;
    segments: SponsorSegment[];
    revision: number;
    updatedAt: string;
  };
  events: Activity[];
  revision: number;
  publicRoomsEnabled: boolean;
}
export function currentPlaybackPosition(playback: Snapshot['playback'], receivedAt: number, now = Date.now()): number {
  if (playback.status !== 'playing') return playback.position;
  // Media advances `rate` seconds per wall-clock second, so scale the elapsed time by
  // the playback rate. Rate defaults to 1 for snapshots that predate the field.
  return playback.position + (Math.max(0, now - receivedAt) / 1000) * (playback.rate || 1);
}
export function parseYouTube(input: string): string | null {
  const value = input.trim();
  if (/^[A-Za-z0-9_-]{11}$/.test(value)) return value;
  try {
    const u = new URL(value);
    if (u.hostname === 'youtu.be') return valid(u.pathname.slice(1));
    if (u.hostname.endsWith('youtube.com'))
      return valid(u.searchParams.get('v') ?? u.pathname.split('/').filter(Boolean).at(-1) ?? '');
  } catch {}
  return null;
}
function valid(v: string) {
  return /^[A-Za-z0-9_-]{11}$/.test(v) ? v : null;
}
export function formatActivity(e: Activity) {
  const who = e.actorName || 'Someone';
  const title = String(e.payload?.title || 'a video');
  const position = Number(e.payload?.position || 0);
  const time = `${Math.floor(position / 60)}:${String(Math.floor(position % 60)).padStart(2, '0')}`;
  switch (e.type) {
    case 'member.joined':
      return `${who} joined the room`;
    case 'member.left':
      return `${who} left the room`;
    case 'player.play':
      return `${who} played the video`;
    case 'player.pause':
      return `${who} paused the video`;
    case 'player.seek':
      return `${who} jumped to ${time}`;
    case 'player.rate': {
      const rate = Number(e.payload?.rate || 1);
      return `${who} set the speed to ${rate}×`;
    }
    case 'queue.add':
      return `${who} added “${title}” to the queue`;
    case 'media.activated':
      return `${who} started “${title}”`;
    case 'queue.remove':
      return `${who} removed a video`;
    case 'queue.reorder':
      return `${who} reordered the queue`;
    case 'queue.skip':
      return `${who} skipped to the next video`;
    case 'role.admin_granted':
      return `${who} granted admin access`;
    case 'role.admin_removed':
      return `${who} removed admin access`;
    case 'member.kicked':
      return `${who} kicked a participant`;
    case 'member.banned':
      return `${who} banned a participant`;
    case 'permission.changed':
      return `${who} changed a permission`;
    case 'room.visibility':
      return `${who} changed the room to ${String(e.payload?.visibility ?? '').replace('_', '-')}`;
    case 'room.sponsorblock':
      return `${who} turned SponsorBlock ${e.payload?.enabled ? 'on' : 'off'}`;
    case 'room.transfer':
      return `${who} transferred room ownership`;
    case 'room.created':
      return `${who} created the room`;
    default:
      return `${who} updated the room`;
  }
}
