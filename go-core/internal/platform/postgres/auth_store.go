package postgres

import (
	"context"
	"errors"
	"strconv"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"
	"github.com/jackc/pgx/v5/pgxpool"
	"github.com/meysam81/go-auth/storage"
)

var errUnsupportedAuthStorage = errors.New("auth storage operation is not configured")

// AuthStore adapts the existing users table to go-auth storage interfaces.
// Password hashes are kept in users.password_hash; no additional table is
// required while the application only issues access tokens.
type AuthStore struct {
	db *pgxpool.Pool
}

var _ storage.UserStore = (*AuthStore)(nil)
var _ storage.CredentialStore = (*AuthStore)(nil)

func NewAuthStore(db *pgxpool.Pool) *AuthStore {
	return &AuthStore{db: db}
}

func (s *AuthStore) CreateUser(ctx context.Context, user *storage.User) error {
	var id int64
	var createdAt time.Time
	err := s.db.QueryRow(ctx, `
		INSERT INTO users (email, password_hash)
		VALUES ($1, '')
		RETURNING id, created_at
	`, user.Email).Scan(&id, &createdAt)
	if err != nil {
		if isUniqueViolation(err) {
			return storage.ErrAlreadyExists
		}
		return err
	}

	// basic.Authenticator stores the password hash immediately after this call.
	// It passes the same user pointer, so replace its temporary generated ID with
	// the BIGSERIAL identifier returned by Postgres.
	user.ID = strconv.FormatInt(id, 10)
	user.Provider = "basic"
	user.CreatedAt = createdAt
	return nil
}

func (s *AuthStore) GetUserByID(ctx context.Context, id string) (*storage.User, error) {
	userID, err := parseUserID(id)
	if err != nil {
		return nil, storage.ErrNotFound
	}

	return s.getUser(ctx, `SELECT id, email, created_at FROM users WHERE id = $1`, userID)
}

func (s *AuthStore) GetUserByEmail(ctx context.Context, email string) (*storage.User, error) {
	return s.getUser(ctx, `SELECT id, email, created_at FROM users WHERE email = $1`, email)
}

// The current users schema uses email as the only login identifier.
func (s *AuthStore) GetUserByUsername(context.Context, string) (*storage.User, error) {
	return nil, storage.ErrNotFound
}

func (s *AuthStore) UpdateUser(ctx context.Context, user *storage.User) error {
	userID, err := parseUserID(user.ID)
	if err != nil {
		return storage.ErrNotFound
	}

	result, err := s.db.Exec(ctx, `UPDATE users SET email = $2 WHERE id = $1`, userID, user.Email)
	if err != nil {
		if isUniqueViolation(err) {
			return storage.ErrAlreadyExists
		}
		return err
	}
	if result.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *AuthStore) DeleteUser(ctx context.Context, id string) error {
	userID, err := parseUserID(id)
	if err != nil {
		return storage.ErrNotFound
	}

	result, err := s.db.Exec(ctx, `DELETE FROM users WHERE id = $1`, userID)
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *AuthStore) StorePasswordHash(ctx context.Context, userID string, hash []byte) error {
	id, err := parseUserID(userID)
	if err != nil {
		return storage.ErrNotFound
	}

	result, err := s.db.Exec(ctx, `UPDATE users SET password_hash = $2 WHERE id = $1`, id, string(hash))
	if err != nil {
		return err
	}
	if result.RowsAffected() == 0 {
		return storage.ErrNotFound
	}
	return nil
}

func (s *AuthStore) GetPasswordHash(ctx context.Context, userID string) ([]byte, error) {
	id, err := parseUserID(userID)
	if err != nil {
		return nil, storage.ErrNotFound
	}

	var hash string
	err = s.db.QueryRow(ctx, `SELECT password_hash FROM users WHERE id = $1`, id).Scan(&hash)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}
	return []byte(hash), nil
}

func (s *AuthStore) StoreWebAuthnCredential(context.Context, string, *storage.WebAuthnCredential) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) GetWebAuthnCredentials(context.Context, string) ([]*storage.WebAuthnCredential, error) {
	return nil, errUnsupportedAuthStorage
}

func (s *AuthStore) UpdateWebAuthnCredential(context.Context, *storage.WebAuthnCredential) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) DeleteWebAuthnCredential(context.Context, []byte) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) StorePasswordResetToken(context.Context, string, string, time.Time) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) ValidatePasswordResetToken(context.Context, string) (string, error) {
	return "", errUnsupportedAuthStorage
}

func (s *AuthStore) DeletePasswordResetToken(context.Context, string) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) StoreEmailVerificationToken(context.Context, string, string, time.Time) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) ValidateEmailVerificationToken(context.Context, string) (string, error) {
	return "", errUnsupportedAuthStorage
}

func (s *AuthStore) DeleteEmailVerificationToken(context.Context, string) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) StoreTOTPSecret(context.Context, string, string, []string) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) GetTOTPSecret(context.Context, string) (string, []string, error) {
	return "", nil, errUnsupportedAuthStorage
}

func (s *AuthStore) DeleteTOTPSecret(context.Context, string) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) UseBackupCode(context.Context, string, string) error {
	return errUnsupportedAuthStorage
}

func (s *AuthStore) getUser(ctx context.Context, query string, arg any) (*storage.User, error) {
	var id int64
	var email string
	var createdAt time.Time
	err := s.db.QueryRow(ctx, query, arg).Scan(&id, &email, &createdAt)
	if errors.Is(err, pgx.ErrNoRows) {
		return nil, storage.ErrNotFound
	}
	if err != nil {
		return nil, err
	}

	return &storage.User{
		ID:        strconv.FormatInt(id, 10),
		Email:     email,
		Provider:  "basic",
		CreatedAt: createdAt,
	}, nil
}

func parseUserID(id string) (int64, error) {
	return strconv.ParseInt(id, 10, 64)
}

func isUniqueViolation(err error) bool {
	var pgErr *pgconn.PgError
	return errors.As(err, &pgErr) && pgErr.Code == "23505"
}
