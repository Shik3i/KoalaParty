package app

import (
	"database/sql"
	"encoding/json"
	"net/http"
	"strconv"
	"time"
)

func (a *application) adminStats(w http.ResponseWriter, r *http.Request, p principal) {
	var totalAccounts int
	if err := a.db.QueryRow("SELECT count(*) FROM accounts").Scan(&totalAccounts); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}

	var totalRooms int
	if err := a.db.QueryRow("SELECT count(*) FROM rooms WHERE deleted_at IS NULL").Scan(&totalRooms); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}

	onlineUsers := a.hub.onlineCount()

	a.hub.mu.RLock()
	activeRoomsList := make([]map[string]any, 0)
	for rid, clients := range a.hub.rooms {
		unique := make(map[string]struct{})
		for c := range clients {
			unique[c.identity] = struct{}{}
		}

		var title sql.NullString
		_ = a.db.QueryRow("SELECT m.title FROM playback_states p JOIN media_items m ON m.id=p.current_media_id WHERE p.room_id=?", rid).Scan(&title)

		activeRoomsList = append(activeRoomsList, map[string]any{
			"roomId":       rid,
			"label":        roomLabel(rid),
			"viewerCount":  len(unique),
			"currentVideo": title.String,
		})
	}
	a.hub.mu.RUnlock()

	writeJSON(w, 200, map[string]any{
		"totalAccounts": totalAccounts,
		"totalRooms":    totalRooms,
		"onlineUsers":   onlineUsers,
		"activeRooms":   activeRoomsList,
	})
}

func (a *application) adminSettings(w http.ResponseWriter, r *http.Request, p principal) {
	if r.Method == "GET" {
		a.mu.RLock()
		defer a.mu.RUnlock()
		writeJSON(w, 200, map[string]any{
			"sessionTTL":        a.sessionTTL.String(),
			"activityMaxAge":    a.activityMaxAge.String(),
			"activityMaxEvents": a.activityMaxEvents,
			"roomMaxIdle":       a.roomMaxIdle.String(),
			"publicRooms":       a.publicRooms,
		})
		return
	}

	var in struct {
		SessionTTL        string `json:"sessionTTL"`
		ActivityMaxAge    string `json:"activityMaxAge"`
		ActivityMaxEvents int    `json:"activityMaxEvents"`
		RoomMaxIdle       string `json:"roomMaxIdle"`
		PublicRooms       bool   `json:"publicRooms"`
	}
	if !decode(w, r, &in) {
		return
	}

	sessionTTL, err := time.ParseDuration(in.SessionTTL)
	if err != nil || sessionTTL <= 0 {
		problem(w, 400, "invalid_settings", "Session TTL must be a positive duration.")
		return
	}
	activityMaxAge, err := time.ParseDuration(in.ActivityMaxAge)
	if err != nil || activityMaxAge <= 0 {
		problem(w, 400, "invalid_settings", "Activity Max Age must be a positive duration.")
		return
	}
	roomMaxIdle, err := time.ParseDuration(in.RoomMaxIdle)
	if err != nil || roomMaxIdle <= 0 {
		problem(w, 400, "invalid_settings", "Room Max Idle must be a positive duration.")
		return
	}
	if in.ActivityMaxEvents < 10 {
		problem(w, 400, "invalid_settings", "Activity Max Events must be at least 10.")
		return
	}

	tx, err := a.db.Begin()
	if err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	defer tx.Rollback()

	upsert := func(key, val string) error {
		_, err := tx.Exec("INSERT INTO settings(key, value) VALUES(?, ?) ON CONFLICT(key) DO UPDATE SET value=excluded.value", key, val)
		return err
	}

	if err := upsert("session_ttl", in.SessionTTL); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	if err := upsert("activity_max_age", in.ActivityMaxAge); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	if err := upsert("activity_max_events", strconv.Itoa(in.ActivityMaxEvents)); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	if err := upsert("room_max_idle", in.RoomMaxIdle); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	if err := upsert("public_rooms", strconv.FormatBool(in.PublicRooms)); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}

	a.mu.Lock()
	a.sessionTTL = sessionTTL
	a.activityMaxAge = activityMaxAge
	a.activityMaxEvents = in.ActivityMaxEvents
	a.roomMaxIdle = roomMaxIdle
	a.publicRooms = in.PublicRooms
	a.mu.Unlock()

	w.WriteHeader(204)
}

func (a *application) adminReports(w http.ResponseWriter, r *http.Request, p principal) {
	rows, err := a.db.Query("SELECT id, room_id, reason, metadata_json, created_at FROM room_reports WHERE resolved_at IS NULL ORDER BY created_at DESC")
	if err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	defer rows.Close()

	reports := []map[string]any{}
	for rows.Next() {
		var id, room, reason, metadata, created string
		if err := rows.Scan(&id, &room, &reason, &metadata, &created); err != nil {
			problem(w, 500, "database_error", err.Error())
			return
		}
		var snapshot any
		_ = json.Unmarshal([]byte(metadata), &snapshot)

		reports = append(reports, map[string]any{
			"id":        id,
			"roomId":    room,
			"roomLabel": roomLabel(room),
			"reason":    reason,
			"metadata":  snapshot,
			"createdAt": created,
		})
	}
	if err := rows.Err(); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	writeJSON(w, 200, reports)
}

func (a *application) resolveReport(w http.ResponseWriter, r *http.Request, p principal) {
	reportID := r.PathValue("reportId")
	result, err := a.db.Exec("UPDATE room_reports SET resolved_at=CURRENT_TIMESTAMP WHERE id=? AND resolved_at IS NULL", reportID)
	if err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		problem(w, 404, "report_not_found", "Report not found or already resolved.")
		return
	}
	w.WriteHeader(204)
}

func (a *application) delistReport(w http.ResponseWriter, r *http.Request, p principal) {
	reportID := r.PathValue("reportId")

	tx, err := a.db.Begin()
	if err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	defer tx.Rollback()

	var room string
	if err := tx.QueryRow("SELECT room_id FROM room_reports WHERE id=?", reportID).Scan(&room); err != nil {
		problem(w, 404, "report_not_found", "Report not found.")
		return
	}

	result, err := tx.Exec("UPDATE room_reports SET resolved_at=CURRENT_TIMESTAMP, delisted_at=CURRENT_TIMESTAMP WHERE id=? AND resolved_at IS NULL", reportID)
	if err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	if changed, _ := result.RowsAffected(); changed == 0 {
		problem(w, 400, "report_already_resolved", "Report already resolved.")
		return
	}

	_, err = tx.Exec("UPDATE rooms SET visibility='unlisted', updated_at=CURRENT_TIMESTAMP WHERE id=?", room)
	if err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}

	if err := tx.Commit(); err != nil {
		problem(w, 500, "database_error", err.Error())
		return
	}
	w.WriteHeader(204)
}
