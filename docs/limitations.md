# Known limitations and extension points

- One backend instance and an in-process WebSocket hub. A future event-bus adapter is required before horizontal scaling.
- YouTube is the only media provider. New items use a server-generated placeholder until the optional oEmbed lookup supplies trusted metadata. The privacy-enhanced player loads when a room opens; no analytics or advertising SDK is included.
- Optional SponsorBlock skipping is a second external dependency (`sponsor.ajay.app`), fetched server-side via the privacy-preserving hash-prefix endpoint and cached. Segment data is CC BY-NC-SA 4.0 and attributed in-app; the feature is opt-out per room and via `KOALAPARTY_SPONSORBLOCK=false`.
- No chat. The activity UI reserves a future tab; chat should use a separate `chat_messages` table.
- No email or password recovery. Anonymous identity loss is intentionally unrecoverable until linked to an account.
- Individual permission enforcement and updates are implemented and covered end-to-end; the initial room UI exposes role, kick, and ban controls but not the complete per-capability editor or ban list.
- Reports are persisted and can be listed, resolved, or used to immediately delist a room through the protected web admin console or host-only operator CLI. Public discovery is disabled by default.
- Automated player synchronization uses UI/API state and a mockable boundary. Real YouTube iframe behavior requires the documented manual smoke test.

Future adapters: PostgreSQL repository, event bus, additional `MediaProvider` implementations, chat storage, and recovery credentials.
