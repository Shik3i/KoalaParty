export type DiagnosticValue = string | number | boolean | null;

export interface DiagnosticEvent {
  at: string;
  source: string;
  event: string;
  details: Record<string, DiagnosticValue>;
}

export function createDiagnosticEvent(
  source: string,
  event: string,
  details: Record<string, DiagnosticValue> = {},
): DiagnosticEvent {
  return { at: new Date().toISOString(), source, event, details };
}

export function formatDiagnosticEvents(
  events: DiagnosticEvent[],
  context: Record<string, DiagnosticValue> = {},
): string {
  return JSON.stringify(
    {
      generatedAt: new Date().toISOString(),
      context,
      events,
    },
    null,
    2,
  );
}
