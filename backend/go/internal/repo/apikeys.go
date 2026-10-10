package repo

import (
	"context"
	"fmt"
	"time"

	"waifuapi/internal/auth"
)

// GetUserByAPIKeyHash resolves a hashed API key to its owner.
//
// It is the auth.UserLookup implementation, kept here because it is a database
// concern. Expiry is checked in SQL so an expired key cannot authenticate even
// if the caller forgets to check it.
func (r *Repo) GetUserByAPIKeyHash(ctx context.Context, hash string) (auth.APIKeyOwner, error) {
	const query = `
        SELECT u."Id", u."Name", u."DiscordId", u."Role", u."IsBlacklisted"
        FROM "ApiKey" k
        JOIN "User" u ON u."Id" = k."UserId"
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
        UPDATE "ApiKey" SET "LastUsedAt" = $1 WHERE "KeyHash" = $2`, at, hash)
	if err != nil {
		return fmt.Errorf("record api key use: %w", err)
	}
	return nil
}

// CreateAPIKey stores a new hashed key for a user and returns the row ID.
func (r *Repo) CreateAPIKey(ctx context.Context, userID int64, description, hash string, expiresAt *time.Time) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx, `
        INSERT INTO "ApiKey" ("UserId", "Description", "KeyHash", "CreatedAt", "ExpirationDate")
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
        DELETE FROM "ApiKey" WHERE "Id" = $1 AND "UserId" = $2`, keyID, userID)
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
        FROM "ReviewTask"
        WHERE "SubmitterId" = $1 AND "Kind" = $2 AND "Status" = $3`,
		userID, int(1 /* KindEdit */), int(0 /* TaskPending */)).Scan(&count)
	if err != nil {
		return 0, fmt.Errorf("count open edits: %w", err)
	}
	return count, nil
}
