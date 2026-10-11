package repo

import (
	"context"
	"errors"
	"fmt"

	"github.com/jackc/pgx/v5"
)

// Album is a row of the Album table plus its item count.
type Album struct {
	ID          int64
	UserID      int64
	Name        string
	Description string
	IsDefault   bool
	ImageCount  int
}

// ListAlbums returns a page of a user's albums.
func (r *Repo) ListAlbums(ctx context.Context, userID int64, offset, limit int) ([]Album, int64, error) {
	var total int64
	if err := r.db.QueryRow(ctx,
		`SELECT COUNT(*) FROM "Albums" WHERE "UserId" = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count albums: %w", err)
	}

	rows, err := r.db.Query(ctx, `
        SELECT a."Id", a."UserId", a."Name", a."Description", a."IsDefault",
               (SELECT COUNT(*) FROM "AlbumItems" ai WHERE ai."AlbumId" = a."Id")
        FROM "Albums" a
        WHERE a."UserId" = $1
        ORDER BY a."IsDefault" DESC, a."Name" ASC
        LIMIT $2 OFFSET $3`, userID, limit, offset)
	if err != nil {
		return nil, 0, fmt.Errorf("list albums: %w", err)
	}
	defer rows.Close()

	albums := make([]Album, 0, limit)
	for rows.Next() {
		album, err := scanAlbum(rows)
		if err != nil {
			return nil, 0, err
		}
		albums = append(albums, album)
	}
	return albums, total, rows.Err()
}

func scanAlbum(row rowScanner) (Album, error) {
	var album Album
	err := row.Scan(&album.ID, &album.UserID, &album.Name,
		&album.Description, &album.IsDefault, &album.ImageCount)
	if err != nil {
		return Album{}, fmt.Errorf("scan album: %w", err)
	}
	return album, nil
}

// GetAlbum fetches one album owned by the given user.
//
// The owner is part of the WHERE clause, so a request for another account's
// album reports as not found rather than as forbidden.
func (r *Repo) GetAlbum(ctx context.Context, albumID, userID int64) (Album, error) {
	return scanAlbum(r.db.QueryRow(ctx, `
        SELECT a."Id", a."UserId", a."Name", a."Description", a."IsDefault",
               (SELECT COUNT(*) FROM "AlbumItems" ai WHERE ai."AlbumId" = a."Id")
        FROM "Albums" a
        WHERE a."Id" = $1 AND a."UserId" = $2`, albumID, userID))
}

// CreateAlbum inserts a new album.
func (r *Repo) CreateAlbum(ctx context.Context, userID int64, name, description string) (Album, error) {
	return scanAlbum(r.db.QueryRow(ctx, `
        INSERT INTO "Albums" ("UserId", "Name", "Description", "IsDefault")
        VALUES ($1, $2, $3, false)
        RETURNING "Id", "UserId", "Name", "Description", "IsDefault",
                  (SELECT COUNT(*) FROM "AlbumItems" ai WHERE ai."AlbumId" = "Albums"."Id")`,
		userID, name, description))
}

// UpdateAlbum renames an album.
func (r *Repo) UpdateAlbum(ctx context.Context, albumID, userID int64, name, description string) (Album, error) {
	album, err := scanAlbum(r.db.QueryRow(ctx, `
        UPDATE "Albums"
        SET "Name" = $1, "Description" = $2
        WHERE "Id" = $3 AND "UserId" = $4
        RETURNING "Id", "UserId", "Name", "Description", "IsDefault",
                  (SELECT COUNT(*) FROM "AlbumItems" ai WHERE ai."AlbumId" = "Albums"."Id")`,
		name, description, albumID, userID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return Album{}, ErrNotFound
		}
		return Album{}, fmt.Errorf("update album: %w", err)
	}
	return album, nil
}

// DeleteAlbum removes an album.
func (r *Repo) DeleteAlbum(ctx context.Context, albumID, userID int64) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM "Albums" WHERE "Id" = $1 AND "UserId" = $2`, albumID, userID)
	if err != nil {
		return fmt.Errorf("delete album: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// AddAlbumItem attaches an image to an album.
//
// Adding the same image twice is not an error, so the client can retry without
// special handling. Pending images are accepted: a user should be able to file
// away their own upload before a moderator has looked at it.
func (r *Repo) AddAlbumItem(ctx context.Context, albumID, userID, imageID int64) error {
	// Confirm ownership first so a missing album is distinguishable from a
	// missing image.
	if _, err := r.GetAlbum(ctx, albumID, userID); err != nil {
		return err
	}

	_, err := r.db.Exec(ctx, `
        INSERT INTO "AlbumItems" ("AlbumId", "ImageId", "AddedAt")
        VALUES ($1, $2, now())
        ON CONFLICT DO NOTHING`, albumID, imageID)
	if err != nil {
		return fmt.Errorf("add album item: %w", err)
	}
	return nil
}

// RemoveAlbumItem detaches an image from an album.
func (r *Repo) RemoveAlbumItem(ctx context.Context, albumID, userID, imageID int64) error {
	if _, err := r.GetAlbum(ctx, albumID, userID); err != nil {
		return err
	}

	tag, err := r.db.Exec(ctx,
		`DELETE FROM "AlbumItems" WHERE "AlbumId" = $1 AND "ImageId" = $2`, albumID, imageID)
	if err != nil {
		return fmt.Errorf("remove album item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// GetAlbumTx fetches an album within a transaction.
func GetAlbumTx(ctx context.Context, tx Tx, albumID, userID int64) (Album, error) {
	return scanAlbum(tx.QueryRow(ctx, `
        SELECT a."Id", a."UserId", a."Name", a."Description", a."IsDefault",
               (SELECT COUNT(*) FROM "AlbumItems" ai WHERE ai."AlbumId" = a."Id")
        FROM "Albums" a
        WHERE a."Id" = $1 AND a."UserId" = $2`, albumID, userID))
}

// EnsureDefaultAlbum returns the user's default album, creating it if absent.
//
// The frontend treats "save to favorites" as an action that should always
// succeed, so the album is created on demand rather than requiring the user to
// make one first.
func (r *Repo) EnsureDefaultAlbum(ctx context.Context, userID int64, name string) (int64, error) {
	id, err := r.GetDefaultAlbumID(ctx, userID)
	if err == nil {
		return id, nil
	}
	if err != ErrNotFound {
		return 0, err
	}

	// The insert clears any existing default first, so a concurrent create
	// cannot leave two albums marked default.
	var newID int64
	err = r.db.QueryRow(ctx, `
        WITH cleared AS (
            UPDATE "Albums" SET "IsDefault" = false WHERE "UserId" = $1 RETURNING 1
        )
        INSERT INTO "Albums" ("UserId", "Name", "Description", "IsDefault")
        VALUES ($1, $2, '', true)
        RETURNING "Id"`, userID, name).Scan(&newID)
	if err != nil {
		return 0, fmt.Errorf("ensure default album: %w", err)
	}
	return newID, nil
}

// GetDefaultAlbumID returns the id of the user's default album.
func (r *Repo) GetDefaultAlbumID(ctx context.Context, userID int64) (int64, error) {
	var id int64
	err := r.db.QueryRow(ctx,
		`SELECT "Id" FROM "Albums" WHERE "UserId" = $1 AND "IsDefault" = true LIMIT 1`,
		userID).Scan(&id)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, ErrNotFound
		}
		return 0, fmt.Errorf("get default album: %w", err)
	}
	return id, nil
}
