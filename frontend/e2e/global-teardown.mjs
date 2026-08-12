export default async function globalTeardown() {
  try {
    await fetch('http://127.0.0.1:4187/api/e2e/shutdown', { method: 'POST' });
  } catch {
    // The server may already have stopped after a failed startup or test.
  }
}
