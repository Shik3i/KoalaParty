import { describe, expect, it } from 'vitest';
import { createDiagnosticEvent, formatDiagnosticEvents } from './diagnostics';

describe('diagnostics', () => {
  it('creates exportable local events without payloads', () => {
    const event = createDiagnosticEvent('player', 'error', { code: 5, videoId: 'safe-id' });
    const output = formatDiagnosticEvents([event], { roomId: 'room', online: true });
    const parsed = JSON.parse(output) as { events: unknown[]; context: { roomId: string } };
    expect(parsed.context.roomId).toBe('room');
    expect(parsed.events).toHaveLength(1);
    expect(output).not.toContain('payload');
  });
});
