export interface RecentRoom {
  id: string;
  label: string;
  title: string;
  visitedAt: string;
}

const storageKey = 'koalaparty.recent-rooms.v1';
const roomID = /^[A-Z2-7]{16}$/;
const limit = 5;

function valid(value: unknown): value is RecentRoom {
  if (!value || typeof value !== 'object') return false;
  const room = value as Partial<RecentRoom>;
  return (
    typeof room.id === 'string' &&
    roomID.test(room.id) &&
    typeof room.label === 'string' &&
    room.label.length > 0 &&
    room.label.length <= 80 &&
    typeof room.title === 'string' &&
    room.title.length <= 200 &&
    typeof room.visitedAt === 'string' &&
    !Number.isNaN(Date.parse(room.visitedAt))
  );
}

export function recentRooms(): RecentRoom[] {
  try {
    const parsed: unknown = JSON.parse(localStorage.getItem(storageKey) ?? '[]');
    if (!Array.isArray(parsed)) return [];
    return parsed
      .filter(valid)
      .sort((a, b) => Date.parse(b.visitedAt) - Date.parse(a.visitedAt))
      .slice(0, limit);
  } catch {
    return [];
  }
}

export function rememberRoom(room: Omit<RecentRoom, 'visitedAt'>): RecentRoom[] {
  const entry = { ...room, id: room.id.toUpperCase(), visitedAt: new Date().toISOString() };
  if (!valid(entry)) return recentRooms();
  const rooms = [entry, ...recentRooms().filter((candidate) => candidate.id !== entry.id)].slice(0, limit);
  persist(rooms);
  return rooms;
}

export function forgetRoom(id: string): RecentRoom[] {
  const rooms = recentRooms().filter((room) => room.id !== id.toUpperCase());
  persist(rooms);
  return rooms;
}

export function reconcileRecentRooms(previews: Array<{ id: string; label: string; title: string }>): RecentRoom[] {
  const available = new Map(previews.map((preview) => [preview.id.toUpperCase(), preview]));
  const rooms = recentRooms()
    .filter((room) => available.has(room.id))
    .map((room) => {
      const preview = available.get(room.id)!;
      return { ...room, label: preview.label, title: preview.title };
    });
  persist(rooms);
  return rooms;
}

function persist(rooms: RecentRoom[]) {
  try {
    localStorage.setItem(storageKey, JSON.stringify(rooms));
  } catch {
    // Recent-room shortcuts are optional when browser storage is unavailable.
  }
}

export function resetRecentRoomsForTests() {
  try {
    localStorage.removeItem(storageKey);
  } catch {
    // Storage may be unavailable in restricted browser contexts.
  }
}
