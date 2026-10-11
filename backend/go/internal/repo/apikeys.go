package repo

import (
	"context"
	"errors"
	"fmt"
	"time"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/auth"
	"waifuapi/internal/store/models"
)

// GetUserByAPIKeyHash resolves a hashed API key to its owner.
//
// It is the auth.UserLookup implementation, kept here because it is a database
// concern. Expiry is checked in SQL so an expired key cannot authenticate even
// if the caller forgets to check it.
func (r *Repo) GetUserByAPIKeyHash(ctx context.Context, hash string) (auth.APIKeyOwner, error) {
	const query = `
        SELECT u."Id", u."Name", u."DiscordId", u."Role", u."IsBlacklisted"
        FROM "ApiKeys" k
        JOIN "Users" u ON u."Id" = k."UserId"
        WHERE k."KeyHash" = $1
          AND (k."ExpirationDate" IS NULL OR k."ExpirationDate" > now())`

	var owner auth.APIKeyOwner
	err := r.db.QueryRow(ctx, query, hash).
		Scan(&owner.UserID, &owner.Name, &owner.DiscordID, &owner.Role, &owner.Blacklisted)
	if err != nil {
		return auth.APIKeyOwner{}, err
	}
	return owner, nil
}

// RecordAPIKeyUse stamps the last-used time on a key.
//
// Failures are returned but callers may ignore them: losing a timestamp is far
// less important than failing a legitimate request, so this is called in a
// best-effort way.
func (r *Repo) RecordAPIKeyUse(ctx context.Context, hash string, at time.Time) error {
	_, err := r.db.Exec(ctx, `
        UPDATE "ApiKeys" SET "LastUsedAt" = $1 WHERE "KeyHash" = $2`, at, hash)
	if err != nil {
		return fmt.Errorf("record api key use: %w", err)
	}
	return nil
}

// CreateAPIKey stores a new hashed key for a user and returns the row ID.
func (r *Repo) CreateAPIKey(ctx context.Context, userID int64, description, hash string, expiresAt *time.Time) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
        INSERT INTO "ApiKeys" ("UserId", "Description", "KeyHash", "CreatedAt", "ExpirationDate")
        VALUES ($1, $2, $3, now(), $4)
        RETURNING "Id"`,
		userID, description, hash, expiresAt).Scan(&id)
	if err != nil {
		return 0, fmt.Errorf("create api key: %w", err)
	}
	return id, nil
}

// DeleteAPIKey removes a key owned by the given user.
//
// The user id is part of the WHERE clause so a key cannot be deleted by someone
// other than its owner, which is the authorisation check expressed in SQL.
func (r *Repo) DeleteAPIKey(ctx context.Context, keyID, userID int64) error {
	tag, err := r.db.Exec(ctx, `
        DELETE FROM "ApiKeys" WHERE "Id" = $1 AND "UserId" = $2`, keyID, userID)
	if err != nil {
		return fmt.Errorf("delete api key: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// CountUserOpenEdits returns how many pending edit submissions a user has.
//
// The C# service caps this via Review__MaxOpenEditsPerUser to stop one user
// flooding the review queue.
func (r *Repo) CountUserOpenEdits(ctx context.Context, userID int64) (int64, error) {
	var count int64
	err := r.db.QueryRow(ctx, `
        SELECT COUNT(*)
        FROM "ReviewTasks"
        WHERE "SubmitterId" = $1 AND "Kind" = $2 AND "Status" = $3`,
		userID, int(1 /* KindEdit */), int(0 /* TaskPending */)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count open edits: %w", err)
	}
	return count, nil
}

// APIKeyRecord describes a key being created.
type APIKeyRecord struct {
	ID          int64
	CreatedAt   time.Time
	Description string
	ExpiresAt   *time.Time
}

// ListAPIKeys returns a user's keys, newest first.
func (r *Repo) ListAPIKeys(ctx context.Context, userID int64) ([]models.APIKey, error) {
	rows, err := r.db.Query(ctx, `
        SELECT "Id", "UserId", "Description", "CreatedAt", "LastUsedAt", "ExpirationDate"
        FROM "ApiKeys"
        WHERE "UserId" = $1
        ORDER BY "CreatedAt" DESC`, userID)
	if err != nil {
		return nil, fmt.Errorf("list api keys: %w", err)
	}
	defer rows.Close()

	keys := make([]models.APIKey, 0, 4)
	for rows.Next() {
		var key models.APIKey
		if err := rows.Scan(&key.ID, &key.UserID, &key.Description,
			&key.CreatedAt, &key.LastUsedAt, &key.ExpiresAt); err != nil {
			return nil, fmt.Errorf("scan api key: %w", err)
		}
		keys = append(keys, key)
	}
	return keys, rows.Err()
}

// CreateAPIKeyRecord stores a new key and returns it.
//
// expiresAt is a formatted timestamp string rather than a time.Time so the
// caller can control the exact instant; it is parsed back here so the column
// receives a real timestamp.
func (r *Repo) CreateAPIKeyRecord(ctx context.Context, userID int64, description, hash string, expiresAt *string) (APIKeyRecord, error) {
	var record APIKeyRecord

	var parsedExpires *time.Time
	if expiresAt != nil {
		when, err := time.Parse("2006-01-02T15:04:05Z", *expiresAt)
		if err != nil {
			return APIKeyRecord{}, fmt.Errorf("parse expiration: %w", err)
		}
		parsedExpires = &when
	}

	err := r.db.QueryRow(ctx, `
        INSERT INTO "ApiKeys" ("UserId", "Description", "KeyHash", "CreatedAt", "ExpirationDate")
        VALUES ($1, $2, $3, now(), $4)
        RETURNING "Id", "CreatedAt", "Description", "ExpirationDate"`,
		userID, description, hash, parsedExpires).
		Scan(&record.ID, &record.CreatedAt, &record.Description, &record.ExpiresAt)
	if err != nil {
		return APIKeyRecord{}, fmt.Errorf("create api key record: %w", err)
	}
	return record, nil
}

// UpdateAPIKeyDescription renames a key.
//
// Ownership is part of the WHERE clause, so a key belonging to another account
// reports as not found rather than as forbidden.
func (r *Repo) UpdateAPIKeyDescription(ctx context.Context, keyID, userID int64, description string) (models.APIKey, error) {
	var key models.APIKey
	err := r.db.QueryRow(ctx, `
        UPDATE "ApiKeys" SET "Description" = $1
        WHERE "Id" = $2 AND "UserId" = $3
        RETURNING "Id", "UserId", "Description", "CreatedAt", "LastUsedAt", "ExpirationDate"`,
		description, keyID, userID).
		Scan(&key.ID, &key.UserID, &key.Description,
			&key.CreatedAt, &key.LastUsedAt, &key.ExpiresAt)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.APIKey{}, ErrNotFound
		}
		return models.APIKey{}, fmt.Errorf("update api key: %w", err)
	}
	return key, nil
}
