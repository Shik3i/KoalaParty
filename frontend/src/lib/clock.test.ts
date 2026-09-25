import { describe, expect, it } from 'vitest';
import { anchorTime, measureClockOffset, offsetFromSamples } from './clock';

describe('clock offset estimation', () => {
  it('uses the lowest-latency sample and its midpoint', () => {
    expect(
      offsetFromSamples([
        { sentAt: 1_000, receivedAt: 1_400, serverNow: 6_000 },
        { sentAt: 2_000, receivedAt: 2_040, serverNow: 7_120 },
      ]),
    ).toBe(5_100);
    expect(offsetFromSamples([])).toBeNull();
    expect(offsetFromSamples([{ sentAt: 5, receivedAt: 1, serverNow: 9 }])).toBeNull();
  });

  it('measures through the time endpoint and tolerates failures', async () => {
    let clock = 0;
    let call = 0;
    const fetchImpl = (async () => {
      call += 1;
      clock += 10; // request travels
      if (call === 1) throw new Error('offline');
      const serverNow = clock + 10_000;
      clock += 10; // response travels
      return new Response(JSON.stringify({ now: serverNow }));
    }) as unknown as typeof fetch;
    const offset = await measureClockOffset(fetchImpl, () => clock, 3);
    expect(offset).toBe(10_000);
  });

  it('anchors snapshots on the server clock but never in the future', () => {
    expect(anchorTime(20_000, 10_000, 10_150)).toBe(10_000);
    expect(anchorTime(undefined, 10_000, 10_150)).toBe(10_150);
    expect(anchorTime(20_000, null, 10_150)).toBe(10_150);
    expect(anchorTime(30_000, 10_000, 10_150)).toBe(10_150);
    expect(anchorTime(20_000, 10_000, 20_000)).toBe(20_000);
  });
});
