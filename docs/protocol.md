# WebSocket protocol

Connect to `/api/rooms/{roomId}/ws` with the session cookie. The server sends and broadcasts `snapshot` messages. Commands use `{ "type": "player.play", "requestId": "...", "expectedRevision": 3, "payload": {} }`; failures use `error` messages. Every state-changing command contains the latest room-wide `expectedRevision`; stale commands are rejected. Playback maintains a separate playback revision. Heartbeats and drift corrections are not activity events.
