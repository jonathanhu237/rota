-- +goose Up
-- +goose StatementBegin

WITH ranked_active_leaves AS (
    SELECT
        id,
        state,
        ROW_NUMBER() OVER (
            PARTITION BY requester_user_id, requester_assignment_id, occurrence_date
            ORDER BY
                CASE state WHEN 'approved' THEN 0 ELSE 1 END,
                created_at,
                id
        ) AS duplicate_rank
    FROM shift_change_requests
    WHERE leave_id IS NOT NULL
      AND state IN ('pending', 'approved')
)
UPDATE shift_change_requests AS request
SET
    state = 'invalidated',
    decided_at = COALESCE(request.decided_at, NOW())
FROM ranked_active_leaves AS ranked
WHERE request.id = ranked.id
  AND ranked.duplicate_rank > 1
  AND ranked.state = 'pending';

CREATE UNIQUE INDEX shift_change_requests_active_leave_occurrence_uidx
    ON shift_change_requests (
        requester_user_id,
        requester_assignment_id,
        occurrence_date
    )
    WHERE leave_id IS NOT NULL
      AND state IN ('pending', 'approved');

-- +goose StatementEnd

-- +goose Down
-- +goose StatementBegin

DROP INDEX IF EXISTS shift_change_requests_active_leave_occurrence_uidx;

-- +goose StatementEnd
