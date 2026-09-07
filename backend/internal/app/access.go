package app

import (
	"context"
	"database/sql"
)

// Shared by metadata responses, commands and snapshot recipients. Roles never bypass room
// eligibility; only the owner bypasses invitations and friendship checks.
// The query must bind rooms as r and identities as i.
const roomAccessSQL = `r.deleted_at IS NULL
	AND NOT EXISTS (SELECT 1 FROM room_bans b WHERE b.room_id=r.id AND b.revoked_at IS NULL
		AND (b.identity_id=i.id OR (b.account_id IS NOT NULL AND b.account_id=i.account_id)))
	AND (r.visibility IN ('unlisted','public') OR (i.account_id IS NOT NULL AND (
		i.id=r.owner_identity_id
		OR (r.visibility='private' AND EXISTS (SELECT 1 FROM room_invites inv WHERE inv.room_id=r.id AND inv.account_id=i.account_id))
		OR (r.visibility='friends_only' AND EXISTS (
			SELECT 1 FROM identities owner JOIN friendships f ON f.status='accepted'
			AND ((f.requester_account_id=owner.account_id AND f.addressee_account_id=i.account_id)
			OR (f.addressee_account_id=owner.account_id AND f.requester_account_id=i.account_id))
			WHERE owner.id=r.owner_identity_id)))))`

type accessQueryer interface {
	QueryRowContext(context.Context, string, ...any) *sql.Row
}

func roomAccess(ctx context.Context, q accessQueryer, room, identity string) bool {
	var allowed bool
	err := q.QueryRowContext(ctx, `SELECT `+roomAccessSQL+` FROM rooms r CROSS JOIN identities i WHERE r.id=? AND i.id=?`, room, identity).Scan(&allowed)
	return err == nil && allowed
}

func roleAndAllowed(ctx context.Context, q accessQueryer, room, identity, capability string) (string, bool) {
	var role string
	var allowed bool
	err := q.QueryRowContext(ctx, `SELECT m.role,
		CASE WHEN m.role IN ('owner','admin') THEN 1 ELSE coalesce((SELECT allowed FROM room_permissions
		WHERE room_id=r.id AND identity_id=i.id AND permission=?),1) END
		FROM room_members m JOIN rooms r ON r.id=m.room_id JOIN identities i ON i.id=m.identity_id
		WHERE r.id=? AND i.id=? AND `+roomAccessSQL, capability, room, identity).Scan(&role, &allowed)
	return role, err == nil && allowed
}

func (a *application) refreshRoomAccess(ctx context.Context, room string) {
	if s, err := a.snapshot(ctx, room, ""); err == nil {
		a.hub.broadcast(room, s)
	} else {
		// A failed refresh must not leave a revoked viewer receiving future data.
		a.hub.disconnectRoom(room)
	}
}

func (a *application) refreshFriendRoomAccess(ctx context.Context, first, second string) {
	rows, err := a.db.QueryContext(ctx, `SELECT r.id FROM rooms r JOIN identities i ON i.id=r.owner_identity_id
		WHERE r.visibility='friends_only' AND r.deleted_at IS NULL AND i.account_id IN (?,?)`, first, second)
	if err != nil {
		return
	}
	var rooms []string
	for rows.Next() {
		var room string
		if rows.Scan(&room) == nil {
			rooms = append(rooms, room)
		}
	}
	rows.Close()
	for _, room := range rooms {
		a.refreshRoomAccess(ctx, room)
	}
}
