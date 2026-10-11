package repo

import (
	"context"
	"fmt"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/store/models"
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
		`SELECT COUNT(*) FROM "Album" WHERE "UserId" = $1`, userID).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count albums: %w", err)
	}

	rows, err := r.db.Query(ctx, `
        SELECT a."Id", a."UserId", a."Name", a."Description", a."IsDefault",
               (SELECT COUNT(*) FROM "AlbumItem" ai WHERE ai."AlbumId" = a."Id")
        FROM "Album" a
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
               (SELECT COUNT(*) FROM "AlbumItem" ai WHERE ai."AlbumId" = a."Id")
        FROM "Album" a
        WHERE a."Id" = $1 AND a."UserId" = $2`, albumID, userID))
}

// CreateAlbum inserts a new album.
func (r *Repo) CreateAlbum(ctx context.Context, userID int64, name, description string) (Album, error) {
	return scanAlbum(r.db.QueryRow(ctx, `
        INSERT INTO "Album" ("UserId", "Name", "Description", "IsDefault")
        VALUES ($1, $2, $3, false)
        RETURNING "Id", "UserId", "Name", "Description", "IsDefault",
                  (SELECT COUNT(*) FROM "AlbumItem" ai WHERE ai."AlbumId" = "Album"."Id")`,
		userID, name, description))
}

// UpdateAlbum renames an album.
func (r *Repo) UpdateAlbum(ctx context.Context, albumID, userID int64, name, description string) (Album, error) {
	album, err := scanAlbum(r.db.QueryRow(ctx, `
        UPDATE "Album"
        SET "Name" = $1, "Description" = $2
        WHERE "Id" = $3 AND "UserId" = $4
        RETURNING "Id", "UserId", "Name", "Description", "IsDefault",
                  (SELECT COUNT(*) FROM "AlbumItem" ai WHERE ai."AlbumId" = "Album"."Id")`,
		name, description, albumID, userID))
	if err != nil {
		if err == pgx.ErrNoRows {
			return Album{}, ErrNotFound
		}
		return Album{}, fmt.Errorf("update album: %w", err)
	}
	return album, nil
}

// DeleteAlbum removes an album.
func (r *Repo) DeleteAlbum(ctx context.Context, albumID, userID int64) error {
	tag, err := r.db.Exec(ctx,
		`DELETE FROM "Album" WHERE "Id" = $1 AND "UserId" = $2`, albumID, userID)
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
        INSERT INTO "AlbumItem" ("AlbumId", "ImageId", "AddedAt")
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
		`DELETE FROM "AlbumItem" WHERE "AlbumId" = $1 AND "ImageId" = $2`, albumID, imageID)
	if err != nil {
		return fmt.Errorf("remove album item: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetDefaultAlbum marks one album as the default and clears the rest.
//
// Both writes share the caller's transaction so two concurrent requests cannot
// leave two defaults behind.
func (r *Repo) SetDefaultAlbum(ctx context.Context, tx pgx.Tx, albumID, userID int64) error {
	if _, err := GetAlbumTx(ctx, tx, albumID, userID); err != nil {
		return err
	}

	if _, err := tx.Exec(ctx,
		`UPDATE "Album" SET "IsDefault" = false WHERE "UserId" = $1`, userID); err != nil {
		return fmt.Errorf("clear default album: %w", err)
	}

	if _, err := tx.Exec(ctx,
		`UPDATE "Album" SET "IsDefault" = true WHERE "Id" = $1 AND "UserId" = $2`,
		albumID, userID); err != nil {
		return fmt.Errorf("set default album: %w", err)
	}
	return nil
}

// GetAlbumTx fetches an album within a transaction.
func GetAlbumTx(ctx context.Context, tx pgx.Tx, albumID, userID int64) (Album, error) {
	return scanAlbum(tx.QueryRow(ctx, `
        SELECT a."Id", a."UserId", a."Name", a."Description", a."IsDefault",
               (SELECT COUNT(*) FROM "AlbumItem" ai WHERE ai."AlbumId" = a."Id")
        FROM "Album" a
        WHERE a."Id" = $1 AND a."UserId" = $2`, albumID, userID))
}

// GetAlbumImages returns the images in an album.
//
// Ordering follows when each image was added, which is what makes an album read
// in the order the user arranged it.
func (r *Repo) GetAlbumImages(ctx context.Context, albumID, userID int64) ([]models.Image, error) {
	rows, err := r.db.Query(ctx, `
        SELECT `+imageColumns+`
        FROM "AlbumItem" ai
        JOIN "Image" i ON i."Id" = ai."ImageId"
        JOIN "Album" a ON a."Id" = ai."AlbumId"
        WHERE ai."AlbumId" = $1 AND a."UserId" = $2
        ORDER BY ai."AddedAt" ASC`, albumID, userID)
	if err != nil {
		return nil, fmt.Errorf("get album images: %w", err)
	}
	defer rows.Close()

	images := make([]models.Image, 0, 16)
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			return nil, err
		}
		images = append(images, image)
	}
	return images, rows.Err()
}
