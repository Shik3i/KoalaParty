import { describe, expect, it } from 'vitest';
import { currentPlaybackPosition, formatActivity, parseYouTube } from './room';

describe('YouTube input', () => {
  it.each([
    ['dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
    ['https://youtu.be/dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
    ['https://www.youtube.com/watch?v=dQw4w9WgXcQ', 'dQw4w9WgXcQ'],
  ])('parses %s', (input, expected) => expect(parseYouTube(input)).toBe(expected));
  it('rejects unrelated URLs', () => expect(parseYouTube('https://example.com/video')).toBeNull());
});
describe('activity formatting', () => {
  it('renders structured seek events', () =>
    expect(
      formatActivity({ id: '1', actorName: 'Moss', type: 'player.seek', payload: { position: 763 }, createdAt: '' }),
    ).toBe('Moss jumped to 12:43'));
});
describe('playback position', () => {
  const playback = { media: null, status: 'playing', position: 12.5, revision: 1, updatedAt: '' };
  it('advances a playing snapshot from its local receipt time', () =>
    expect(currentPlaybackPosition(playback, 1_000, 4_250)).toBe(15.75));
  it('does not advance paused playback', () =>
    expect(currentPlaybackPosition({ ...playback, status: 'paused' }, 1_000, 4_250)).toBe(12.5));
  it('ignores a clock that moved backwards', () => expect(currentPlaybackPosition(playback, 4_250, 1_000)).toBe(12.5));
});
