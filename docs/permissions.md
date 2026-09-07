# Permission and access model

Roles are authoritative: `owner > admin > member`. Owners have all capabilities and cannot be modified, kicked, banned, or demoted by another participant. Admins receive all normal and management capabilities over non-owners. Member capability overrides are stored per room and identity; absent overrides use collaborative defaults.

Default member capabilities: `playback.play_pause`, `playback.seek`, `media.play_now`, `queue.add`, `queue.remove`, `queue.reorder`, `queue.vote`, and `queue.skip`. Management capabilities are restricted to owners/admins; deletion and ownership transfer are owner-exclusive.

Visibility is enforced before room state is returned or a WebSocket is upgraded. Unlisted rooms accept the link. Public rooms require an account-linked owner before listing. Private rooms require an account and explicit invitation. Friends-only rooms require an accepted friendship with the owner's account. Admin status does not change the friends-only eligibility rule.

Public, private, and friends-only visibility require an account-linked owner. Anonymous owners cannot switch into a mode that would reject their own rejoin. Invitation changes and membership departure authorize against the same database transaction as their mutation.

Kick closes active sockets and removes room presence; rejoining remains possible. Ban records the persistent identity and linked account where available, closes active sockets, and blocks reconnects. Anonymous users can evade bans by deleting browser storage; no fingerprinting or permanent IP ban is used.

Eligibility also applies to existing memberships: visibility changes, revoked invitations, and removed or blocked friendships close ineligible sockets and remove current metadata from room lists and previews. Commands recheck eligibility and capabilities inside their transaction. Transferring a private room retains an invitation for the former owner because the transfer keeps them as an admin; friends-only eligibility still requires the new owner's friendship.
