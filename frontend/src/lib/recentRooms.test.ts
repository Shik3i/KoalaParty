import { beforeEach, describe, expect, it, vi } from 'vitest';
import { forgetRoom, recentRooms, rememberRoom, resetRecentRoomsForTests } from './recentRooms';

describe('recent rooms', () => {
  beforeEach(() => {
    localStorage.clear();
    resetRecentRoomsForTests();
    vi.useFakeTimers();
    vi.setSystemTime(new Date('2026-07-21T12:00:00Z'));
  });

  it('keeps the five most recently visited unique rooms', () => {
    for (let i = 0; i < 6; i++) {
      vi.setSystemTime(new Date(Date.UTC(2026, 6, 21, 12, i)));
      rememberRoom({ id: `AAAAAAAAAAAAAAA${i + 2}`, label: `Room ${i}`, title: `Video ${i}` });
    }
    const rooms = recentRooms();
    expect(rooms).toHaveLength(5);
    expect(rooms[0].label).toBe('Room 5');
    expect(rooms.some((room) => room.label === 'Room 0')).toBe(false);
  });

  it('moves a revisited room to the front without duplicating it', () => {
    rememberRoom({ id: 'AAAAAAAAAAAAAAA2', label: 'First', title: 'Old title' });
    vi.setSystemTime(new Date('2026-07-21T13:00:00Z'));
    rememberRoom({ id: 'AAAAAAAAAAAAAAA2', label: 'First', title: 'New title' });
    expect(recentRooms()).toMatchObject([{ id: 'AAAAAAAAAAAAAAA2', title: 'New title' }]);
  });

  it('forgets a room and ignores malformed storage', () => {
    rememberRoom({ id: 'AAAAAAAAAAAAAAA2', label: 'First', title: '' });
    expect(forgetRoom('aaaaaaaaaaaaaaa2')).toEqual([]);
    localStorage.setItem('koalaparty.recent-rooms.v1', '{broken');
    expect(recentRooms()).toEqual([]);
  });
});
