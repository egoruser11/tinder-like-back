package seed

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgxpool"
	"golang.org/x/crypto/bcrypt"
)

const DemoPassword = "Password123!"

type account struct {
	email     string
	fullName  string
	birthday  string
	gender    int16
	city      string
	latitude  float64
	longitude float64
}

var accounts = []account{
	{"demo@example.com", "Демо Пользователь", "1995-06-15", 1, "Москва", 55.7558, 37.6176},
	{"alina@example.com", "Алина", "1997-03-21", 2, "Москва", 55.7522, 37.6156},
	{"daria@example.com", "Дарья", "1994-11-08", 2, "Москва", 55.7610, 37.6312},
	{"sofia@example.com", "София", "1998-07-13", 2, "Москва", 55.7412, 37.6081},
	{"maria@example.com", "Мария", "1993-01-27", 2, "Москва", 55.7731, 37.6054},
	{"elena@example.com", "Елена", "1996-09-02", 2, "Москва", 55.7305, 37.6402},
	{"nikita@example.com", "Никита", "1992-05-17", 1, "Москва", 55.7443, 37.6219},
	{"ivan@example.com", "Иван", "1991-02-10", 1, "Санкт-Петербург", 59.9343, 30.3351},
}

// Load inserts deterministic local data. It is idempotent and may be run again
// without duplicating accounts or profiles.
func Load(ctx context.Context, db *pgxpool.Pool) error {
	passwordHash, err := bcrypt.GenerateFromPassword([]byte(DemoPassword), bcrypt.DefaultCost)
	if err != nil {
		return fmt.Errorf("hash seed password: %w", err)
	}

	tx, err := db.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin seed transaction: %w", err)
	}
	defer tx.Rollback(ctx)

	ids := make(map[string]int64, len(accounts))
	for _, account := range accounts {
		userID, err := ensureUser(ctx, tx, account.email, string(passwordHash))
		if err != nil {
			return err
		}
		ids[account.email] = userID

		if _, err := tx.Exec(ctx, `
			INSERT INTO profiles (
				user_id, full_name, birthday, gender, bio, city,
				latitude, longitude, is_active, last_activity_at
			)
			VALUES ($1, $2, $3::date, $4, $5, $6, $7, $8, TRUE, now())
			ON CONFLICT (user_id) DO UPDATE SET
				full_name = EXCLUDED.full_name,
				birthday = EXCLUDED.birthday,
				gender = EXCLUDED.gender,
				bio = EXCLUDED.bio,
				city = EXCLUDED.city,
				latitude = EXCLUDED.latitude,
				longitude = EXCLUDED.longitude,
				is_active = TRUE,
				last_activity_at = now(),
				updated_at = now()
		`, userID, account.fullName, account.birthday, account.gender,
			"Тестовый профиль для разработки ленты", account.city, account.latitude, account.longitude); err != nil {
			return fmt.Errorf("upsert profile %s: %w", account.email, err)
		}
	}

	demoID := ids["demo@example.com"]
	if _, err := tx.Exec(ctx, `
		INSERT INTO discovery_preferences (user_id, min_age, max_age, gender, max_distance_km)
		VALUES ($1, 18, 50, 2, 50)
		ON CONFLICT (user_id) DO UPDATE SET
			min_age = EXCLUDED.min_age,
			max_age = EXCLUDED.max_age,
			gender = EXCLUDED.gender,
			max_distance_km = EXCLUDED.max_distance_km,
			updated_at = now()
	`, demoID); err != nil {
		return fmt.Errorf("upsert demo preferences: %w", err)
	}

	// One incoming like makes GET /ribbon/likes easy to test once its query is written.
	if _, err := tx.Exec(ctx, `
		INSERT INTO swipes (actor_user_id, target_user_id)
		VALUES ($1, $2)
		ON CONFLICT (actor_user_id, target_user_id) DO NOTHING
	`, ids["alina@example.com"], demoID); err != nil {
		return fmt.Errorf("insert seed like: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit seed transaction: %w", err)
	}
	return nil
}

func ensureUser(ctx context.Context, tx pgx.Tx, email, passwordHash string) (int64, error) {
	var userID int64
	err := tx.QueryRow(ctx, `
		INSERT INTO users (email, password_hash, is_verified, verified_at)
		VALUES ($1, $2, TRUE, $3)
		ON CONFLICT DO NOTHING
		RETURNING id
	`, email, passwordHash, time.Now()).Scan(&userID)
	if err == nil {
		return userID, nil
	}
	if !errors.Is(err, pgx.ErrNoRows) {
		return 0, fmt.Errorf("insert seed user %s: %w", email, err)
	}

	if err := tx.QueryRow(ctx, `SELECT id FROM users WHERE lower(email) = lower($1)`, email).Scan(&userID); err != nil {
		return 0, fmt.Errorf("find seed user %s: %w", email, err)
	}
	return userID, nil
}
