package scenarios

import (
	"context"
	"database/sql"
)

func RunBasic(ctx context.Context, tx *sql.Tx, opts Options) error {
	if _, _, err := insertUsers(ctx, tx, opts, 5); err != nil {
		return err
	}
	if _, err := insertPositions(ctx, tx, 3); err != nil {
		return err
	}
	if _, err := insertTemplate(ctx, tx, "Default Rota", false, opts.Now); err != nil {
		return err
	}
	return nil
}
