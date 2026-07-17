import { describe, expect, it } from 'vitest';
import { formatActivity, parseYouTube } from './room';

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
