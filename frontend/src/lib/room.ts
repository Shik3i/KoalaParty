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

export function reconnectDelay(attempt: number, random = Math.random): number {
  const exponent = Math.max(0, Math.min(6, Math.floor(attempt) - 1));
  const base = Math.min(10_000, 1_000 * 2 ** exponent);
  const jitter = Math.max(0, Math.min(1, random()));
  return Math.round(base * (1 + jitter * 0.3));
}

export function automaticEndDelay(snapshot: Pick<Snapshot, 'members' | 'me'>): number | null {
  const reporters = snapshot.members
    .filter(
      (member) =>
        member.active &&
        (member.role === 'owner' || member.role === 'admin' || member.permissions['queue.skip'] !== false),
    )
    .sort((a, b) => {
      const roleRank = (member: Member) => (member.role === 'owner' ? 0 : member.role === 'admin' ? 1 : 2);
      return roleRank(a) - roleRank(b) || a.identityId.localeCompare(b.identityId);
    });
  const rank = reporters.findIndex((member) => member.identityId === snapshot.me);
  return rank < 0 ? null : rank * 400;
}

export const END_REPORT_LEASE_MS = 15_000;

export function remainingEndReportLease(
  stored: { signature?: string; at?: number } | null,
  signature: string,
  now = Date.now(),
): number {
  if (stored?.signature !== signature || typeof stored.at !== 'number') return 0;
  const age = now - stored.at;
  if (age < 0 || age >= END_REPORT_LEASE_MS) return 0;
  return END_REPORT_LEASE_MS - age;
}

export function filterQueue(items: QueueItem[], value: string): QueueItem[] {
  const query = value.trim().toLocaleLowerCase();
  if (!query) return items;
  return items.filter(
    (item) =>
      item.media.title.toLocaleLowerCase().includes(query) || item.media.providerId.toLocaleLowerCase().includes(query),
  );
}

export function participantNameParts(displayName: string): { badge: string; label: string } {
  const normalized = displayName.trim();
  const emoji = normalized.match(/^(\p{Extended_Pictographic}️?)\s+(.+)$/u);
  if (emoji) return { badge: emoji[1], label: emoji[2] };
  // Anonymous identities created before v0.4.1 used "Koala NNN" without an
  // emoji. Render their intended animal badge without mutating the persistent
  // identity or requiring every legacy participant to reconnect first.
  if (/^Koala \d{3}$/u.test(normalized)) return { badge: '🐨', label: normalized };
  return { badge: (normalized[0] ?? '?').toUpperCase(), label: normalized || displayName };
}
export function parseYouTube(input: string): string | null {
  const value = input.trim();
  if (/^[A-Za-z0-9_-]{11}$/.test(value)) return value;
  try {
    const u = new URL(value);
    if (u.hostname === 'youtu.be' || u.hostname === 'www.youtu.be') return valid(u.pathname.slice(1));
    if (
      u.hostname === 'youtube.com' ||
      u.hostname.endsWith('.youtube.com') ||
      u.hostname === 'youtube-nocookie.com' ||
      u.hostname.endsWith('.youtube-nocookie.com')
    )
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
    case 'media.ended':
      return `${who} finished the video`;
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
    case 'member.permission':
      return `${who} changed a permission`;
    case 'queue.shuffle':
      return `${who} shuffled the queue`;
    case 'queue.loop':
      return `${who} turned queue looping ${e.payload?.enabled ? 'on' : 'off'}`;
    case 'member.unbanned':
      return `${who} removed a room ban`;
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
