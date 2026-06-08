package auth

import (
	"context"
	"database/sql"
	"errors"
)

type Repository struct {
	db *sql.DB
}

func NewRepository(db *sql.DB) *Repository {
	return &Repository{db: db}
}

func (r *Repository) FindByNickname(ctx context.Context, nickname string) (User, bool, error) {
	const query = `
SELECT id, nickname, avatar, token
  FROM users
 WHERE nickname = ?
 LIMIT 1`
	var user User
	var avatar sql.NullString
	err := r.db.QueryRowContext(ctx, query, nickname).Scan(&user.ID, &user.Nickname, &avatar, &user.Token)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	user.Avatar = nullableStringPtr(avatar)
	return user, true, nil
}

func (r *Repository) FindByToken(ctx context.Context, token string) (User, bool, error) {
	const query = `
SELECT id, nickname, avatar, token
  FROM users
 WHERE token = ?
 LIMIT 1`
	var user User
	var avatar sql.NullString
	err := r.db.QueryRowContext(ctx, query, token).Scan(&user.ID, &user.Nickname, &avatar, &user.Token)
	if errors.Is(err, sql.ErrNoRows) {
		return User{}, false, nil
	}
	if err != nil {
		return User{}, false, err
	}
	user.Avatar = nullableStringPtr(avatar)
	return user, true, nil
}

func (r *Repository) CreateMockUser(ctx context.Context, nickname string, avatar *string, kind string) (User, error) {
	tx, err := r.db.BeginTx(ctx, nil)
	if err != nil {
		return User{}, err
	}
	defer tx.Rollback()

	const insertSQL = `
INSERT INTO users (nickname, avatar, token)
VALUES (?, ?, '__placeholder__')`
	result, err := tx.ExecContext(ctx, insertSQL, nickname, avatar)
	if err != nil {
		return User{}, err
	}
	id, err := result.LastInsertId()
	if err != nil {
		return User{}, err
	}
	token := MockToken(kind, id)

	const updateSQL = `
UPDATE users
   SET token = ?
 WHERE id = ?`
	if _, err := tx.ExecContext(ctx, updateSQL, token, id); err != nil {
		return User{}, err
	}
	if err := tx.Commit(); err != nil {
		return User{}, err
	}

	return User{ID: id, Nickname: nickname, Avatar: avatar, Token: token}, nil
}

func nullableStringPtr(value sql.NullString) *string {
	if !value.Valid {
		return nil
	}
	return &value.String
}
