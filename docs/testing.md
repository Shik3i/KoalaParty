# Testing strategy

`make verify` runs backend tests and static analysis plus frontend formatting, lint, type checks, unit tests, and production build. CI also builds the Docker image. Browser synchronization tests use a mock media provider; they do not claim to test YouTube.

## Manual YouTube smoke test

1. Open one room in two browser profiles and select **Start watching** in both.
2. Add `https://www.youtube.com/watch?v=dQw4w9WgXcQ` and use **Play now**.
3. Confirm privacy-enhanced iframe loading, play/pause/seek synchronization, drift correction, queue auto-advance, and a clear unavailable-video state.
