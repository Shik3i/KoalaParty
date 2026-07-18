package app

import (
	"context"
	"time"
)

func (a *application) maintenanceLoop(ctx context.Context) {
	a.runMaintenance(ctx)
	ticker := time.NewTicker(6 * time.Hour)
	defer ticker.Stop()
	for {
		select {
		case <-ctx.Done():
			return
		case <-ticker.C:
			a.runMaintenance(ctx)
		}
	}
}
func (a *application) runMaintenance(ctx context.Context) {
	cutoff := time.Now().Add(-a.getActivityMaxAge()).UTC().Format("2006-01-02 15:04:05")
	_, _ = a.db.ExecContext(ctx, "DELETE FROM room_events WHERE created_at < ?", cutoff)
	_, _ = a.db.ExecContext(ctx, `DELETE FROM room_events WHERE id IN (SELECT id FROM (SELECT id,row_number() OVER(PARTITION BY room_id ORDER BY created_at DESC) n FROM room_events) WHERE n>?)`, a.getActivityMaxEvents())
	roomCutoff := time.Now().Add(-a.getRoomMaxIdle()).UTC().Format("2006-01-02 15:04:05")
	rows, e := a.db.QueryContext(ctx, "SELECT id FROM rooms WHERE deleted_at IS NULL AND last_active_at < ?", roomCutoff)
	if e == nil {
		var ids []string
		for rows.Next() {
			var id string
			_ = rows.Scan(&id)
			ids = append(ids, id)
		}
		rows.Close()
		for _, id := range ids {
			if !a.hub.activeRoom(id) {
				_, _ = a.db.ExecContext(ctx, "UPDATE rooms SET deleted_at=CURRENT_TIMESTAMP WHERE id=? AND last_active_at < ?", id, roomCutoff)
			}
		}
	}
	_, _ = a.db.ExecContext(ctx, "DELETE FROM sessions WHERE expires_at < CURRENT_TIMESTAMP")
}
