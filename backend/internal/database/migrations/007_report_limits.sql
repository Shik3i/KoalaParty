UPDATE room_reports
SET resolved_at = CURRENT_TIMESTAMP
WHERE resolved_at IS NULL
  AND reporter_identity_id IS NOT NULL
  AND rowid NOT IN (
    SELECT max(rowid)
    FROM room_reports
    WHERE resolved_at IS NULL AND reporter_identity_id IS NOT NULL
    GROUP BY room_id, reporter_identity_id
  );

CREATE UNIQUE INDEX room_reports_pending_reporter_idx
ON room_reports(room_id, reporter_identity_id)
WHERE resolved_at IS NULL AND reporter_identity_id IS NOT NULL;
