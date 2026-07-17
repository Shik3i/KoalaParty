# Permission and access model

Roles are authoritative: `owner > admin > member`. Owners bypass all restrictions and cannot be modified, kicked, banned, or demoted. Admins receive all normal and management capabilities over non-owners. Member capability overrides are stored per room and identity; absent overrides use collaborative defaults.

Default member capabilities: `playback.play_pause`, `playback.seek`, `media.play_now`, `queue.add`, `queue.remove`, `queue.reorder`, and `queue.skip`. Management capabilities are restricted to owners/admins; deletion and future ownership transfer are owner-exclusive.

Visibility is enforced before room state is returned or a WebSocket is upgraded. Unlisted rooms accept the link. Public rooms require an account-linked owner before listing. Private rooms require an account and explicit invitation. Friends-only rooms require an accepted friendship with the owner's account. Admin status does not change the friends-only eligibility rule.

Kick closes active sockets and removes room presence; rejoining remains possible. Ban records the persistent identity and linked account where available, closes active sockets, and blocks reconnects. Anonymous users can evade bans by deleting browser storage; no fingerprinting or permanent IP ban is used.

