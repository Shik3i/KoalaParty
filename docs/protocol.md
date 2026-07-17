# WebSocket protocol

Connect to `/api/rooms/{roomId}/ws` with the session cookie. The server sends `snapshot` first. Commands use `{ "type": "player.play", "requestId": "...", "payload": {} }`; responses use `ack` or `error`. Events carry a monotonically increasing room revision. Playback commands contain `expectedRevision`; stale commands are rejected. Heartbeats and drift corrections are not activity events.

