package repo

import (
	"context"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/store/models"
)

// ---------------------------------------------------------------------------
// Tags
// ---------------------------------------------------------------------------

// ListTags returns accepted tags, optionally filtered by a name substring.
func (r *Repo) ListTags(ctx context.Context, search string, page, pageSize int) ([]models.Tag, int64, error) {
	args := []any{int(models.ReviewAccepted)}
	where := `"ReviewStatus" = $1`

	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		where += fmt.Sprintf(` AND LOWER("Name") LIKE $%d`, len(args))
	}

	var total int64
	countQuery := fmt.Sprintf(`SELECT COUNT(*) FROM "Tag" WHERE %s`, where)
	if err := r.db.QueryRow(ctx, countQuery, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tags: %w", err)
	}

	args = append(args, pageSize)
	limit := len(args)
	args = append(args, page)
	offset := len(args)

	query := fmt.Sprintf(`
        SELECT "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"
        FROM "Tag"
        WHERE %s
        ORDER BY "Name" ASC
        LIMIT $%d OFFSET $%d`, where, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	tags := make([]models.Tag, 0, pageSize)
	for rows.Next() {
		var tag models.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description,
			&tag.CreatorID, &tag.ReviewStatus); err != nil {
			return nil, 0, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate tags: %w", err)
	}
	return tags, total, nil
}

// GetTagsBySlugs resolves slugs to tags, used when attaching tags to an upload.
//
// UploadImage/Command.cs fails the whole upload when any slug is unknown, so
// this returns the set it found and the caller compares lengths rather than
// silently ignoring missing ones.
func (r *Repo) GetTagsBySlugs(ctx context.Context, slugs []string) ([]models.Tag, error) {
	if len(slugs) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"
        FROM "Tag"
        WHERE "Slug" = ANY($1) AND "ReviewStatus" = $2`, slugs, int(models.ReviewAccepted))
	if err != nil {
		return nil, fmt.Errorf("get tags by slug: %w", err)
	}
	defer rows.Close()

	tags := make([]models.Tag, 0, len(slugs))
	for rows.Next() {
		var tag models.Tag
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description,
			&tag.CreatorID, &tag.ReviewStatus); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// GetTagByID fetches a single tag.
func (r *Repo) GetTagByID(ctx context.Context, id int64) (models.Tag, error) {
	var tag models.Tag
	err := r.db.QueryRow(ctx, `
        SELECT "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"
        FROM "Tag" WHERE "Id" = $1`, id).
		Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description,
			&tag.CreatorID, &tag.ReviewStatus)
	if err != nil {
		return models.Tag{}, err
	}
	return tag, nil
}

// CreateTag inserts a new tag and returns it.
func (r *Repo) CreateTag(ctx context.Context, name, slug, description string, creatorID *int64, status models.ReviewStatus) (models.Tag, error) {
	var tag models.Tag
	err := r.db.QueryRow(ctx, `
        INSERT INTO "Tag" ("Name", "Slug", "Description", "CreatorId", "ReviewStatus")
        VALUES ($1, $2, $3, $4, $5)
        RETURNING "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"`,
		name, slug, description, creatorID, int(status)).
		Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description,
			&tag.CreatorID, &tag.ReviewStatus)
	if err != nil {
		return models.Tag{}, fmt.Errorf("create tag: %w", err)
	}
	return tag, nil
}

// GetTagsForImage returns the tags attached to an image.
func (r *Repo) GetTagsForImage(ctx context.Context, imageID int64) ([]models.TagMinimal, error) {
	rows, err := r.db.Query(ctx, `
        SELECT t."Id", t."Name", t."Slug"
        FROM "ImageTag" it
        JOIN "Tag" t ON t."Id" = it."TagId"
        WHERE it."ImageId" = $1
        ORDER BY t."Name" ASC`, imageID)
	if err != nil {
		return nil, fmt.Errorf("get tags for image: %w", err)
	}
	defer rows.Close()

	tags := make([]models.TagMinimal, 0, 8)
	for rows.Next() {
		var tag models.TagMinimal
		if err := rows.Scan(&tag.ID, &tag.Name, &tag.Slug); err != nil {
			return nil, fmt.Errorf("scan image tag: %w", err)
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// ---------------------------------------------------------------------------
// Artists
// ---------------------------------------------------------------------------

// ListArtists returns accepted artists, optionally filtered by name.
func (r *Repo) ListArtists(ctx context.Context, search string, page, pageSize int) ([]models.Artist, int64, error) {
	args := []any{int(models.ReviewAccepted)}
	where := `"ReviewStatus" = $1`

	if search != "" {
		args = append(args, "%"+strings.ToLower(search)+"%")
		where += fmt.Sprintf(` AND LOWER("Name") LIKE $%d`, len(args))
	}

	var total int64
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM "Artist" WHERE %s`, where), args...).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count artists: %w", err)
	}

	args = append(args, pageSize)
	limit := len(args)
	args = append(args, page)
	offset := len(args)

	query := fmt.Sprintf(`
        SELECT "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
               "CreatorId", "ReviewStatus"
        FROM "Artist"
        WHERE %s
        ORDER BY "Name" ASC
        LIMIT $%d OFFSET $%d`, where, limit, offset)

	rows, err := r.db.Query(ctx, query, args...)
	if err != nil {
		return nil, 0, fmt.Errorf("list artists: %w", err)
	}
	defer rows.Close()

	artists := make([]models.Artist, 0, pageSize)
	for rows.Next() {
		artist, err := scanArtist(rows)
		if err != nil {
			return nil, 0, err
		}
		artists = append(artists, artist)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate artists: %w", err)
	}
	return artists, total, nil
}

func scanArtist(row rowScanner) (models.Artist, error) {
	var artist models.Artist
	err := row.Scan(&artist.ID, &artist.Name, &artist.DeviantArt, &artist.Twitter,
		&artist.Patreon, &artist.Pixiv, &artist.CreatorID, &artist.ReviewStatus)
	if err != nil {
		return models.Artist{}, fmt.Errorf("scan artist: %w", err)
	}
	return artist, nil
}

// GetArtistsByIDs resolves artist IDs, used when attaching artists to an upload.
//
// As with tags, the caller compares lengths to detect unknown IDs.
func (r *Repo) GetArtistsByIDs(ctx context.Context, ids []int64) ([]models.Artist, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
               "CreatorId", "ReviewStatus"
        FROM "Artist"
        WHERE "Id" = ANY($1) AND "ReviewStatus" = $2`, ids, int(models.ReviewAccepted))
	if err != nil {
		return nil, fmt.Errorf("get artists by id: %w", err)
	}
	defer rows.Close()

	artists := make([]models.Artist, 0, len(ids))
	for rows.Next() {
		artist, err := scanArtist(rows)
		if err != nil {
			return nil, err
		}
		artists = append(artists, artist)
	}
	return artists, rows.Err()
}

// GetArtistByID fetches a single artist.
func (r *Repo) GetArtistByID(ctx context.Context, id int64) (models.Artist, error) {
	return scanArtist(r.db.QueryRow(ctx, `
        SELECT "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
               "CreatorId", "ReviewStatus"
        FROM "Artist" WHERE "Id" = $1`, id))
}

// GetArtistsForImage returns the artists attached to an image.
func (r *Repo) GetArtistsForImage(ctx context.Context, imageID int64) ([]models.ArtistMinimal, error) {
	rows, err := r.db.Query(ctx, `
        SELECT a."Id", a."Name"
        FROM "ArtistImage" ai
        JOIN "Artist" a ON a."Id" = ai."ArtistId"
        WHERE ai."ImageId" = $1
        ORDER BY a."Name" ASC`, imageID)
	if err != nil {
		return nil, fmt.Errorf("get artists for image: %w", err)
	}
	defer rows.Close()

	artists := make([]models.ArtistMinimal, 0, 4)
	for rows.Next() {
		var artist models.ArtistMinimal
		if err := rows.Scan(&artist.ID, &artist.Name); err != nil {
			return nil, fmt.Errorf("scan image artist: %w", err)
		}
		artists = append(artists, artist)
	}
	return artists, rows.Err()
}

// CreateArtist inserts a new artist and returns it.
func (r *Repo) CreateArtist(ctx context.Context, artist models.Artist) (models.Artist, error) {
	return scanArtist(r.db.QueryRow(ctx, `
        INSERT INTO "Artist" ("Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
                              "CreatorId", "ReviewStatus")
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
                  "CreatorId", "ReviewStatus"`,
		artist.Name, artist.DeviantArt, artist.Twitter, artist.Patreon, artist.Pixiv,
		artist.CreatorID, int(artist.ReviewStatus)))
}

// ---------------------------------------------------------------------------
// Users
// ---------------------------------------------------------------------------

// GetUserByID fetches a user by primary key.
func (r *Repo) GetUserByID(ctx context.Context, id int64) (models.User, error) {
	var user models.User
	err := r.db.QueryRow(ctx, `
        SELECT "Id", "DiscordId", "Name", "AvatarUrl", "Role",
               "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
               "IsBlacklisted", "BlacklistReason"
        FROM "User" WHERE "Id" = $1`, id).
		Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL, &user.Role,
			&user.RequestCount, &user.JWTRequestCount, &user.APIKeyRequestCount,
			&user.IsBlacklisted, &user.BlacklistReason)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

// GetUserByDiscordID fetches a user by their Discord identifier.
func (r *Repo) GetUserByDiscordID(ctx context.Context, discordID string) (models.User, error) {
	var user models.User
	err := r.db.QueryRow(ctx, `
        SELECT "Id", "DiscordId", "Name", "AvatarUrl", "Role",
               "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
               "IsBlacklisted", "BlacklistReason"
        FROM "User" WHERE "DiscordId" = $1`, discordID).
		Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL, &user.Role,
			&user.RequestCount, &user.JWTRequestCount, &user.APIKeyRequestCount,
			&user.IsBlacklisted, &user.BlacklistReason)
	if err != nil {
		return models.User{}, err
	}
	return user, nil
}

// UpsertUserByDiscordID creates or refreshes a user on Discord login.
//
// The role is deliberately NOT overwritten. Promotion is an administrative
// action, and a re-login must not reset it.
//
// This mirrors LoginWithDiscord/Command.cs, which updates name and avatar while
// preserving the existing role.
func (r *Repo) UpsertUserByDiscordID(ctx context.Context, discordID, name string, avatarURL *string) (models.User, error) {
	var user models.User
	err := r.db.QueryRow(ctx, `
        INSERT INTO "User" ("DiscordId", "Name", "AvatarUrl", "Role")
        VALUES ($1, $2, $3, 0)
        ON CONFLICT ("DiscordId") DO UPDATE
            SET "Name" = EXCLUDED."Name",
                "AvatarUrl" = EXCLUDED."AvatarUrl"
        RETURNING "Id", "DiscordId", "Name", "AvatarUrl", "Role",
                  "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
                  "IsBlacklisted", "BlacklistReason"`,
		discordID, name, avatarURL).
		Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL, &user.Role,
			&user.RequestCount, &user.JWTRequestCount, &user.APIKeyRequestCount,
			&user.IsBlacklisted, &user.BlacklistReason)
	if err != nil {
		return models.User{}, fmt.Errorf("upsert user: %w", err)
	}
	return user, nil
}

// ---------------------------------------------------------------------------
// Banner
// ---------------------------------------------------------------------------

// GetActiveBanner returns the enabled banner, if any.
//
// The caller answers 204 when the returned pointer is nil. That distinction
// matters: useBanner.ts in the frontend treats 204 as "no banner" and a 200
// with an empty body as the same thing, but the contract is explicit and should
// not drift.
func (r *Repo) GetActiveBanner(ctx context.Context) (*models.SiteBanner, error) {
	var banner models.SiteBanner
	err := r.db.QueryRow(ctx, `
        SELECT "Id", "Message", "Variant", "IsEnabled", "UpdatedAt", "UpdatedById"
        FROM "SiteBanner"
        WHERE "IsEnabled" = true
        ORDER BY "UpdatedAt" DESC
        LIMIT 1`).
		Scan(&banner.ID, &banner.Message, &banner.Variant,
			&banner.IsEnabled, &banner.UpdatedAt, &banner.UpdatedByID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get active banner: %w", err)
	}
	return &banner, nil
}

// GetBannerSettings returns the banner regardless of enabled state, for
// moderators editing it.
func (r *Repo) GetBannerSettings(ctx context.Context) (*models.SiteBanner, error) {
	var banner models.SiteBanner
	err := r.db.QueryRow(ctx, `
        SELECT "Id", "Message", "Variant", "IsEnabled", "UpdatedAt", "UpdatedById"
        FROM "SiteBanner"
        ORDER BY "Id" ASC
        LIMIT 1`).
		Scan(&banner.ID, &banner.Message, &banner.Variant,
			&banner.IsEnabled, &banner.UpdatedAt, &banner.UpdatedByID)
	if err != nil {
		if err == pgx.ErrNoRows {
			return nil, nil
		}
		return nil, fmt.Errorf("get banner settings: %w", err)
	}
	return &banner, nil
}

// SaveBanner upserts the single banner row and returns the stored state.
func (r *Repo) SaveBanner(ctx context.Context, message string, variant models.BannerVariant, enabled bool, editorID *int64) (models.SiteBanner, error) {
	var banner models.SiteBanner
	err := r.db.QueryRow(ctx, `
        INSERT INTO "SiteBanner" ("Message", "Variant", "IsEnabled", "UpdatedAt", "UpdatedById")
        VALUES ($1, $2, $3, now(), $4)
        ON CONFLICT DO NOTHING
        RETURNING "Id", "Message", "Variant", "IsEnabled", "UpdatedAt", "UpdatedById"`,
		message, int(variant), enabled, editorID).
		Scan(&banner.ID, &banner.Message, &banner.Variant,
			&banner.IsEnabled, &banner.UpdatedAt, &banner.UpdatedByID)

	if err == pgx.ErrNoRows {
		// No row existed, so insert then update the single existing row. The
		// table is intended to hold at most one active banner.
		if _, err := r.db.Exec(ctx, `
            INSERT INTO "SiteBanner" ("Message", "Variant", "IsEnabled", "UpdatedAt", "UpdatedById")
            VALUES ($1, $2, $3, now(), $4)`,
			message, int(variant), enabled, editorID); err != nil {
			return models.SiteBanner{}, fmt.Errorf("insert banner: %w", err)
		}
		if err := r.db.QueryRow(ctx, `
            UPDATE "SiteBanner"
            SET "Message" = $1, "Variant" = $2, "IsEnabled" = $3,
                "UpdatedAt" = now(), "UpdatedById" = $4
            RETURNING "Id", "Message", "Variant", "IsEnabled", "UpdatedAt", "UpdatedById"`,
			message, int(variant), enabled, editorID).
			Scan(&banner.ID, &banner.Message, &banner.Variant,
				&banner.IsEnabled, &banner.UpdatedAt, &banner.UpdatedByID); err != nil {
			return models.SiteBanner{}, fmt.Errorf("update banner: %w", err)
		}
		return banner, nil
	}
	if err != nil {
		return models.SiteBanner{}, fmt.Errorf("save banner: %w", err)
	}
	return banner, nil
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

// GetPublicStats returns the counters shown on the home page.
func (r *Repo) GetPublicStats(ctx context.Context) (models.PublicStats, error) {
	var stats models.PublicStats

	err := r.db.QueryRow(ctx, `
        SELECT
            (SELECT COALESCE(SUM("RequestCount"), 0) FROM "DailyStat"),
            (SELECT COUNT(*) FROM "Image" WHERE "ReviewStatus" = $1),
            (SELECT COUNT(*) FROM "Tag"   WHERE "ReviewStatus" = $1),
            (SELECT COUNT(*) FROM "Artist" WHERE "ReviewStatus" = $1)`,
		int(models.ReviewAccepted)).
		Scan(&stats.TotalRequests, &stats.TotalImages, &stats.TotalTags, &stats.TotalArtists)
	if err != nil {
		return models.PublicStats{}, fmt.Errorf("get public stats: %w", err)
	}
	return stats, nil
}
