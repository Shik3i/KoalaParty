// Estimates the offset between this browser's clock and the server's, NTP-style,
// so a snapshot's playback position can be anchored to the moment the server
// computed it instead of the moment it happened to arrive. Without this, network
// latency and clock skew show up as a constant sync offset per viewer.

export interface ClockSample {
  sentAt: number;
  receivedAt: number;
  serverNow: number;
}

/** Server clock minus client clock in ms, from the lowest-latency sample. */
export function offsetFromSamples(samples: ClockSample[]): number | null {
  const valid = samples.filter(
    (sample) => sample.receivedAt >= sample.sentAt && Number.isFinite(sample.serverNow) && sample.serverNow > 0,
  );
  if (!valid.length) return null;
  const best = valid.reduce((a, b) => (b.receivedAt - b.sentAt < a.receivedAt - a.sentAt ? b : a));
  const midpoint = best.sentAt + (best.receivedAt - best.sentAt) / 2;
  return Math.round(best.serverNow - midpoint);
}

export async function measureClockOffset(
  fetchImpl: typeof fetch = fetch,
  now: () => number = () => Date.now(),
  count = 4,
): Promise<number | null> {
  const samples: ClockSample[] = [];
  for (let i = 0; i < count; i++) {
    try {
      const sentAt = now();
      const response = await fetchImpl('/api/time', { cache: 'no-store' });
      const receivedAt = now();
      if (!response.ok) continue;
      const body = (await response.json()) as { now?: number };
      if (typeof body.now === 'number') samples.push({ sentAt, receivedAt, serverNow: body.now });
    } catch {
      /* A failed probe only lowers the sample count. */
    }
  }
  return offsetFromSamples(samples);
}

/**
 * Converts the server time at which a snapshot was computed into this client's
 * clock. Falls back to the receipt time when no estimate exists or the result is
 * implausible (clock jumps, a sleeping device, a stale estimate).
 */
export function anchorTime(serverTime: number | undefined, offset: number | null, receivedAt: number): number {
  if (!serverTime || offset === null) return receivedAt;
  const anchored = serverTime - offset;
  if (anchored > receivedAt + 50 || receivedAt - anchored > 5_000) return receivedAt;
  return Math.min(anchored, receivedAt);
}
