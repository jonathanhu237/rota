package internal

import (
	"context"
	"database/sql"
)

func WipeAllData(ctx context.Context, tx *sql.Tx) error {
	const query = `
		TRUNCATE TABLE
			audit_logs,
			shift_change_requests,
			assignments,
			availability_submissions,
			user_setup_tokens,
			user_positions,
			template_slot_positions,
			template_slots,
			publications,
			templates,
			positions,
			users
		RESTART IDENTITY CASCADE;
	`
	_, err := tx.ExecContext(ctx, query)
	return err
}
