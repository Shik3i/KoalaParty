# Known limitations and extension points

- One backend instance and an in-process WebSocket hub. A future event-bus adapter is required before horizontal scaling.
- YouTube is the only media provider. Metadata uses the submitted fallback title and standard thumbnail URL; thumbnails and the player load only after explicit playback consent, and no tracking metadata API is called.
- No chat. The activity UI reserves a future tab; chat should use a separate `chat_messages` table.
- No email or password recovery. Anonymous identity loss is intentionally unrecoverable until linked to an account.
- Individual permission enforcement and updates are implemented and covered end-to-end; the initial room UI exposes role, kick, and ban controls but not the complete per-capability editor or ban list.
- Reports are persisted and can be listed, resolved, or used to immediately delist a room through the host-only operator CLI. A protected web dashboard remains future work. Public discovery is disabled by default.
- Automated player synchronization uses UI/API state and a mockable boundary. Real YouTube iframe behavior requires the documented manual smoke test.

Future adapters: PostgreSQL repository, event bus, additional `MediaProvider` implementations, chat storage, recovery credentials, and a web moderation dashboard.
