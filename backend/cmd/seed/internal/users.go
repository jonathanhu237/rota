package internal

import (
	"context"
	"database/sql"

	"golang.org/x/crypto/bcrypt"
)

func InsertUser(
	ctx context.Context,
	tx *sql.Tx,
	email, name, password string,
	isAdmin bool,
) (int64, error) {
	hash, err := bcrypt.GenerateFromPassword([]byte(password), bcrypt.DefaultCost)
	if err != nil {
		return 0, err
	}

	var id int64
	err = tx.QueryRowContext(
		ctx,
		`
			INSERT INTO users (email, password_hash, name, is_admin, status, version)
			VALUES ($1, $2, $3, $4, 'active', 1)
			RETURNING id;
		`,
		email,
		string(hash),
		name,
		isAdmin,
	).Scan(&id)
	return id, err
}
