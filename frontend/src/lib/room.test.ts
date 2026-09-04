import { describe, expect, it } from 'vitest';
import {
  automaticEndDelay,
  currentPlaybackPosition,
  filterQueue,
  formatActivity,
  parseYouTube,
  participantNameParts,
  reconnectDelay,
} from './room';

describe('YouTube input', () => {
  it.each([
    ['dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
    ['https://youtu.be/dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
    ['https://www.youtube.com/watch?v=dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
    ['https://music.youtube.com/watch?v=dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
    ['https://www.youtube.com/shorts/dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
    ['https://www.youtube-nocookie.com/embed/dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
  ])('parses %s', (input, expected) => expect(parseYouTube(input)).toBe(expected));
  it.each(['https://example.com/video', 'https://notyoutube.com/watch?v=dQw4w9WgXcQ'])(
    'rejects unrelated URL %s',
    (input) => expect(parseYouTube(input)).toBeNull(),
  );
});
describe('activity formatting', () => {
  it('renders structured seek events', () =>
    expect(
      formatActivity({ id: '1', actorName: 'Moss', type: 'player.seek', payload: { position: 763 }, createdAt: '' }),
    ).toBe('Moss jumped to 12:43'));
  it.each([
    ['member.permission', { permission: 'queue.add', allowed: false }, 'Moss changed a permission'],
    ['queue.shuffle', {}, 'Moss shuffled the queue'],
    ['queue.loop', { enabled: true }, 'Moss turned queue looping on'],
    ['member.unbanned', {}, 'Moss removed a room ban'],
  ])('renders backend event %s', (type, payload, expected) =>
    expect(formatActivity({ id: type, actorName: 'Moss', type, payload, createdAt: '' })).toBe(expected),
  );
});
describe('playback position', () => {
  const playback = {
    media: null,
    status: 'playing',
    position: 12.5,
    rate: 1,
    segments: [],
    revision: 1,
    updatedAt: '',
  };
  it('advances a playing snapshot from its local receipt time', () =>
    expect(currentPlaybackPosition(playback, 1_000, 4_250)).toBe(15.75));
  it('does not advance paused playback', () =>
    expect(currentPlaybackPosition({ ...playback, status: 'paused' }, 1_000, 4_250)).toBe(12.5));
  it('ignores a clock that moved backwards', () => expect(currentPlaybackPosition(playback, 4_250, 1_000)).toBe(12.5));
  it('scales elapsed time by the playback rate', () =>
    // 3.25s of wall clock at 2× advances the media by 6.5s.
    expect(currentPlaybackPosition({ ...playback, rate: 2 }, 1_000, 4_250)).toBe(19));
  it('treats a missing rate as 1×', () =>
    expect(currentPlaybackPosition({ ...playback, rate: undefined as unknown as number }, 1_000, 4_250)).toBe(15.75));
});

describe('connection and automatic-end coordination', () => {
  it('adds bounded jitter to exponential reconnect backoff', () => {
    expect(reconnectDelay(1, () => 0)).toBe(1_000);
    expect(reconnectDelay(3, () => 1)).toBe(5_200);
    expect(reconnectDelay(20, () => 1)).toBe(13_000);
  });

  it('staggers automatic end reports by active capable member', () => {
    const member = (identityId: string, role: 'owner' | 'admin' | 'member', active = true, allowed = true) => ({
      identityId,
      displayName: identityId,
      role,
      active,
      accountLinked: false,
      permissions: { 'queue.skip': allowed },
    });
    const snapshot = {
      me: 'member-b',
      members: [member('member-b', 'member'), member('owner', 'owner'), member('member-a', 'member')],
    };
    expect(automaticEndDelay(snapshot)).toBe(800);
    expect(automaticEndDelay({ ...snapshot, me: 'owner' })).toBe(0);
    expect(
      automaticEndDelay({
        ...snapshot,
        me: 'blocked',
        members: [...snapshot.members, member('blocked', 'member', true, false)],
      }),
    ).toBeNull();
  });
});

describe('queue filtering', () => {
  const items = [
    { id: '1', media: { id: 'm1', providerId: 'alpha123456', title: 'Forest Walk', thumbnail: '' } },
    { id: '2', media: { id: 'm2', providerId: 'beta1234567', title: 'Ocean Waves', thumbnail: '' } },
  ] as Parameters<typeof filterQueue>[0];
  it('matches title and provider ID case-insensitively', () => {
    expect(filterQueue(items, 'OCEAN')).toEqual([items[1]]);
    expect(filterQueue(items, 'alpha123456')).toEqual([items[0]]);
    expect(filterQueue(items, 'missing')).toEqual([]);
  });
  it('keeps the original ordering for an empty query', () => expect(filterQueue(items, '  ')).toBe(items));
});

describe('participant avatars', () => {
  it('separates modern animal emoji from the visible name', () => {
    expect(participantNameParts('🦭 Gentle Seal')).toEqual({ badge: '🦭', label: 'Gentle Seal' });
  });

  it('restores the koala badge for pre-v0.4.1 anonymous names', () => {
    expect(participantNameParts('Koala 474')).toEqual({ badge: '🐨', label: 'Koala 474' });
  });

  it('keeps the initial fallback for custom names', () => {
    expect(participantNameParts('Forest Friend')).toEqual({ badge: 'F', label: 'Forest Friend' });
  });
});

describe('activity formatting — speed', () => {
  it('renders a rate change', () =>
    expect(
      formatActivity({ id: '2', actorName: 'Moss', type: 'player.rate', payload: { rate: 1.5 }, createdAt: '' }),
    ).toBe('Moss set the speed to 1.5×'));
});

describe('activity formatting — sponsorblock', () => {
  it('renders enabling SponsorBlock', () =>
    expect(
      formatActivity({
        id: '3',
        actorName: 'Moss',
        type: 'room.sponsorblock',
        payload: { enabled: true },
        createdAt: '',
      }),
    ).toBe('Moss turned SponsorBlock on'));
  it('renders disabling SponsorBlock', () =>
    expect(
      formatActivity({
        id: '4',
        actorName: 'Moss',
        type: 'room.sponsorblock',
        payload: { enabled: false },
        createdAt: '',
      }),
    ).toBe('Moss turned SponsorBlock off'));
});
