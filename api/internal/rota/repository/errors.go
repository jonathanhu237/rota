package repository

import (
	"errors"

	"github.com/jackc/pgx/v5/pgconn"
	"github.com/lib/pq"
)

// postgresError normalizes errors from both drivers used by this repository
// layer. Runtime connections use pgx through database/sql, while a few
// embedders and tests still return lib/pq errors. Domain error mapping must
// not depend on which driver produced the same PostgreSQL SQLSTATE.
func postgresError(err error) (code, constraint string, ok bool) {
	var pgxErr *pgconn.PgError
	if errors.As(err, &pgxErr) {
		return string(pgxErr.Code), pgxErr.ConstraintName, true
	}
	var pqErr *pq.Error
	if errors.As(err, &pqErr) {
		return string(pqErr.Code), pqErr.Constraint, true
	}
	return "", "", false
}
