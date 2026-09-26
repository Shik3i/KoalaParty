import { tNow, type MessageKey, type Translate } from '$lib/i18n';
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
  addedBy: string;
  start: number;
  // Placed with "play next": plays before voted items.
  next?: boolean;
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
    skipVotes: number;
    skipNeeded: number;
    skipVoted: boolean;
    startsAt?: number;
    autoPaused?: boolean;
    // The current video's length once someone reported it, else 0.
    duration?: number;
  };
  events: Activity[];
  revision: number;
  publicRoomsEnabled: boolean;
  searchEnabled: boolean;
  serverTime?: number;
  slug?: string;
  mode?: RoomMode;
  waitForAll?: boolean;
  countdownSeconds?: number;
  scheduledAt?: number;
  // Broadcasts carry only the newest events; merge them into the known list.
  eventsPartial?: boolean;
}
export type RoomMode = 'party' | 'cinema' | 'host';
// Adds the newest events of a partial broadcast to the known list, oldest first,
// keeping at most `limit` entries.
export function mergeEvents(known: Activity[], incoming: Activity[], limit = 200): Activity[] {
  const seen = new Set(known.map((event) => event.id));
  const merged = [...known, ...incoming.filter((event) => !seen.has(event.id))];
  return merged.length > limit ? merged.slice(merged.length - limit) : merged;
}
// Reaction palette, in keyboard order (1–9, then 0). Must match the server.
export const REACTION_EMOJIS = ['❤️', '😂', '🔥', '👀', '😮', '👏', '🎉', '😭', '🍿', '😴'];
export interface ChatMessage {
  id: string;
  identityId: string;
  name: string;
  text: string;
  at: string;
}
export type PresenceState = 'playing' | 'paused' | 'buffering' | 'blocked' | 'idle';
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

export type AddMode = 'queue' | 'next' | 'now';
export interface VideoRequest {
  videoId: string;
  start: number;
}
export interface ParsedInput {
  videos: VideoRequest[];
  playlistId: string | null;
}

// Parses YouTube timestamps such as "90", "90s", "1m30s" or "1h2m3s".
export function parseStartTime(raw: string | null): number {
  if (!raw) return 0;
  const value = raw.trim().toLowerCase();
  if (/^\d+(\.\d+)?s?$/.test(value)) return Math.floor(parseFloat(value));
  const match = value.match(/^(?:(\d+)h)?(?:(\d+)m)?(?:(\d+)s)?$/);
  if (!match || !value) return 0;
  return Number(match[1] ?? 0) * 3600 + Number(match[2] ?? 0) * 60 + Number(match[3] ?? 0);
}

function parseLink(token: string): { video: VideoRequest | null; playlistId: string | null } {
  try {
    const u = new URL(token);
    const host = u.hostname.toLowerCase();
    const youtube =
      host === 'youtu.be' ||
      host === 'www.youtu.be' ||
      host === 'youtube.com' ||
      host.endsWith('.youtube.com') ||
      host === 'youtube-nocookie.com' ||
      host.endsWith('.youtube-nocookie.com');
    if (!youtube) return { video: null, playlistId: null };
    const list = u.searchParams.get('list');
    const playlistId = list && /^[A-Za-z0-9_-]{10,64}$/.test(list) ? list : null;
    const id = parseYouTube(token);
    const start = parseStartTime(u.searchParams.get('t') ?? u.searchParams.get('start'));
    const isPlaylistPage = u.pathname.replace(/\/+$/, '') === '/playlist';
    return { video: id && !isPlaylistPage ? { videoId: id, start } : null, playlistId };
  } catch {
    return { video: null, playlistId: null };
  }
}

/**
 * Extracts every YouTube video from pasted text: one link, several links (one per
 * line or separated by spaces), or a bare video ID. A playlist link reports its
 * list ID so the room can offer an import. Plain words are never mistaken for IDs.
 */
export function parseYouTubeInput(input: string): ParsedInput {
  const text = input.trim();
  const result: ParsedInput = { videos: [], playlistId: null };
  if (!text) return result;
  if (/^[A-Za-z0-9_-]{11}$/.test(text) && !/^[a-z]+$/.test(text)) {
    result.videos.push({ videoId: text, start: 0 });
    return result;
  }
  const seen = new Set<string>();
  for (const token of text.split(/[\s,]+/)) {
    if (!/^https?:\/\//i.test(token)) continue;
    const { video, playlistId } = parseLink(token);
    if (playlistId && !result.playlistId) result.playlistId = playlistId;
    if (video && !seen.has(video.videoId)) {
      seen.add(video.videoId);
      result.videos.push(video);
    }
  }
  return result;
}

// True when the text looks like a link or ID rather than a search query.
export function looksLikeLink(input: string): boolean {
  const text = input.trim();
  return (
    /^https?:\/\//i.test(text) ||
    /^(www\.)?(youtube\.com|youtu\.be)\//i.test(text) ||
    parseYouTubeInput(text).videos.length > 0
  );
}

export function formatDuration(seconds: number): string {
  const s = Math.max(0, Math.floor(seconds));
  const h = Math.floor(s / 3600);
  const m = Math.floor((s % 3600) / 60);
  const rest = String(s % 60).padStart(2, '0');
  return h ? `${h}:${String(m).padStart(2, '0')}:${rest}` : `${m}:${rest}`;
}
// Activity event types with a plain "{who} did something" sentence.
const SIMPLE_ACTIVITY: Record<string, MessageKey> = {
  'member.joined': 'activity.joined',
  'member.left': 'activity.left',
  'player.play': 'activity.play',
  'player.pause': 'activity.pause',
  'media.skip_voted': 'activity.skipVoted',
  'media.skip_vote_withdrawn': 'activity.skipVoteWithdrawn',
  'media.vote_skipped': 'activity.voteSkipped',
  'media.ended': 'activity.ended',
  'queue.remove': 'activity.remove',
  'queue.reorder': 'activity.reorder',
  'queue.skip': 'activity.skip',
  'role.admin_granted': 'activity.adminGranted',
  'role.admin_removed': 'activity.adminRemoved',
  'member.kicked': 'activity.kicked',
  'member.banned': 'activity.banned',
  'member.permission': 'activity.permission',
  'queue.shuffle': 'activity.shuffle',
  'member.unbanned': 'activity.unbanned',
  'room.transfer': 'activity.transfer',
  'room.created': 'activity.created',
  'room.slug': 'activity.slug',
  'room.mode': 'activity.mode',
  'room.wait': 'activity.wait',
  'room.countdown': 'activity.countdown',
  'room.schedule': 'activity.schedule',
};

export function formatActivity(e: Activity, t: Translate = tNow) {
  const who = e.actorName || t('activity.someone');
  const title = String(e.payload?.title || t('activity.aVideo'));
  const position = Number(e.payload?.position || 0);
  const time = `${Math.floor(position / 60)}:${String(Math.floor(position % 60)).padStart(2, '0')}`;
  const on = (value: unknown) => (value ? t('activity.on') : t('activity.off'));
  switch (e.type) {
    case 'player.pause':
      return e.payload?.reason === 'wait' ? t('activity.waitPause', { who }) : t('activity.pause', { who });
    case 'player.seek':
      return t('activity.seek', { who, time });
    case 'player.rate':
      return t('activity.rate', { who, rate: Number(e.payload?.rate || 1) });
    case 'queue.add': {
      const count = Number(e.payload?.count || 0);
      if (count > 1) return t('activity.addMany', { who, count });
      return e.payload?.next ? t('activity.addNext', { who, title }) : t('activity.add', { who, title });
    }
    case 'room.rename':
      return e.payload?.name
        ? t('activity.rename', { who, name: String(e.payload.name) })
        : t('activity.renameReset', { who });
    case 'media.activated':
      return t('activity.started', { who, title });
    case 'queue.loop':
      return t('activity.loop', { who, state: on(e.payload?.enabled) });
    case 'room.visibility':
      return t('activity.visibility', { who, visibility: String(e.payload?.visibility ?? '').replace('_', '-') });
    case 'room.sponsorblock':
      return t('activity.sponsorblock', { who, state: on(e.payload?.enabled) });
    default:
      return t(SIMPLE_ACTIVITY[e.type] ?? 'activity.updated', { who });
  }
}

export interface TextSegment {
  text: string;
  seconds?: number;
}

// Splits chat text so timestamps such as "1:23" or "1:02:03" can become buttons
// that jump the whole room to that moment.
export function splitTimestamps(text: string): TextSegment[] {
  const segments: TextSegment[] = [];
  const pattern = /(?<![\d:])(?:(\d{1,2}):)?([0-5]?\d):([0-5]\d)(?![\d:])/g;
  let last = 0;
  for (const match of text.matchAll(pattern)) {
    const index = match.index ?? 0;
    if (index > last) segments.push({ text: text.slice(last, index) });
    const seconds = Number(match[1] ?? 0) * 3600 + Number(match[2]) * 60 + Number(match[3]);
    segments.push({ text: match[0], seconds });
    last = index + match[0].length;
  }
  if (last < text.length) segments.push({ text: text.slice(last) });
  return segments;
}

/** An .ics calendar entry for a scheduled party. */
export function partyCalendar(options: { title: string; url: string; start: number; minutes?: number }): string {
  const stamp = (ms: number) =>
    new Date(ms)
      .toISOString()
      .replace(/[-:]/g, '')
      .replace(/\.\d{3}/, '');
  const escape = (value: string) => value.replace(/[\\;,]/g, (c) => `\\${c}`).replace(/\r?\n/g, '\\n');
  return [
    'BEGIN:VCALENDAR',
    'VERSION:2.0',
    'PRODID:-//KoalaParty//Watch party//EN',
    'BEGIN:VEVENT',
    `UID:${options.start}-${encodeURIComponent(options.url)}@koalaparty`,
    `DTSTAMP:${stamp(Date.now())}`,
    `DTSTART:${stamp(options.start)}`,
    `DTEND:${stamp(options.start + (options.minutes ?? 120) * 60_000)}`,
    `SUMMARY:${escape(options.title)}`,
    `URL:${options.url}`,
    `DESCRIPTION:${escape(options.url)}`,
    'BEGIN:VALARM',
    'TRIGGER:-PT10M',
    'ACTION:DISPLAY',
    `DESCRIPTION:${escape(options.title)}`,
    'END:VALARM',
    'END:VEVENT',
    'END:VCALENDAR',
    '',
  ].join('\r\n');
}

/** Human "in 2 h 5 min" style countdown text parts. */
export function untilParts(ms: number): { days: number; hours: number; minutes: number; seconds: number } {
  const total = Math.max(0, Math.floor(ms / 1000));
  return {
    days: Math.floor(total / 86400),
    hours: Math.floor((total % 86400) / 3600),
    minutes: Math.floor((total % 3600) / 60),
    seconds: total % 60,
  };
}
