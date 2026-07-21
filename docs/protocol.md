# WebSocket protocol

Connect to `/api/rooms/{roomId}/ws` with the session cookie. The server sends and broadcasts `snapshot` messages. Commands use `{ "type": "player.play", "requestId": "...", "expectedRevision": 3, "payload": {} }`; failures use `error` messages. Every state-changing command contains the latest room-wide `expectedRevision`; stale commands are rejected. Playback maintains a separate playback revision. Heartbeats and drift corrections are not activity events.

Snapshots are personalized per WebSocket client through the `me` identity field. REST management endpoints provide account room listing, room deletion/leaving, private invitations, profile/password changes, active-session revocation, and account deletion. Ownership transfer is the revision-protected `room.transfer` command and requires an account-linked target member.

Queue commands also include `queue.vote`, `queue.shuffle`, and `queue.loop`. Votes determine which queued item is selected next, with manual position as the tie-breaker. Snapshots expose vote totals, the current viewer's vote, loop state, and the latest 20 played media items. Duplicate current or queued media is rejected.

`reaction.send` is a revision-free WebSocket message with one allowed emoji. Reactions are rate-limited per connection, broadcast as transient `reaction` messages, and are never written to SQLite or room activity. `POST /api/rooms/previews` accepts at most five room IDs and returns metadata only for rooms associated with the authenticated identity or account; it does not join rooms.
