package repo

import (
	"context"
	"errors"
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/store/models"
)

// This file holds the tag, artist, user and banner queries. They are grouped
// because every one of them resolves a review-status predicate the same way and
// sharing that helper is what keeps the two catalogue endpoints behaving
// identically.

// ---------------------------------------------------------------------------
// Shared catalogue filtering
// ---------------------------------------------------------------------------

// CatalogFilter carries the optional parameters of GET /tags and GET /artists.
type CatalogFilter struct {
	// Name is a case-insensitive substring match on the entity name.
	Name string

	// IncludedIDs and IncludedSlugs switch the endpoint into "fetch exactly
	// these" mode, which ignores the name filter and pagination entirely. Tags
	// accept both; artists accept only IDs, because they have no slug.
	IncludedIDs   []int64
	IncludedSlugs []string

	// ReviewStatus is the visibility filter. A non-moderator asking for anything
	// other than Accepted is refused by the handler before reaching here.
	ReviewStatus models.ReviewStatusFilter

	// ViewerID is the caller, used only for the IncludeMine union.
	ViewerID    int64
	IncludeMine bool

	Page     int
	PageSize int
}

// TagEntry is a tag row plus the derived count the list endpoint reports.
type TagEntry struct {
	models.Tag
	ImageCount int64
}

// ArtistEntry is an artist row plus its accepted-image count.
type ArtistEntry struct {
	models.Artist
	ImageCount int64
}

// catalogWhere builds the review-status and identity predicate shared by both
// catalogue listings.
//
// alias differs between the two tables ("t" for Tag, "a" for Artist) and is the
// only parameter; every fragment is chosen from a fixed set here.
func catalogWhere(filter CatalogFilter, alias string) (string, []any) {
	var conditions []string
	var args []any

	switch filter.ReviewStatus {
	case models.FilterPending:
		conditions = append(conditions, fmt.Sprintf(`%s."ReviewStatus" = $1`, alias))
		args = append(args, int(models.ReviewPending))

	case models.FilterAll:
		// no clause

	default:
		// Accepted, plus the caller's own pending entries when asked. This is
		// why the upload screen can show a tag it just created without waiting
		// for moderation.
		if filter.IncludeMine && filter.ViewerID != 0 {
			conditions = append(conditions, fmt.Sprintf(
				`(%s."ReviewStatus" = $1 OR (%s."ReviewStatus" = $2 AND %s."CreatorId" = $3))`,
				alias, alias, alias))
			args = append(args, int(models.ReviewAccepted), int(models.ReviewPending), filter.ViewerID)
		} else {
			conditions = append(conditions, fmt.Sprintf(`%s."ReviewStatus" = $1`, alias))
			args = append(args, int(models.ReviewAccepted))
		}
	}

	// An explicit id or slug list short-circuits the name filter, which is what
	// the C# handler does when it builds its exact-lookup branch.
	switch {
	case len(filter.IncludedIDs) > 0:
		args = append(args, filter.IncludedIDs)
		conditions = append(conditions, fmt.Sprintf(`%s."Id" = ANY($%d)`, alias, len(args)))
	case len(filter.IncludedSlugs) > 0:
		args = append(args, filter.IncludedSlugs)
		conditions = append(conditions, fmt.Sprintf(`%s."Slug" = ANY($%d)`, alias, len(args)))
	}

	if filter.Name != "" {
		args = append(args, "%"+strings.ToLower(filter.Name)+"%")
		conditions = append(conditions, fmt.Sprintf(`LOWER(%s."Name") LIKE $%d`, alias, len(args)))
	}

	return strings.Join(conditions, " AND "), args
}

// acceptedImageCount renders the correlated subquery behind imageCount.
//
// It counts only accepted images, so a tag carrying a large backlog of
// unreviewed uploads does not outrank one that is actually in use.
func acceptedImageCount(linkTable, foreignKey, alias string) string {
	return fmt.Sprintf(`(SELECT COUNT(*) FROM %q l
            JOIN "Images" i ON i."Id" = l.%q
            WHERE l.%q = %s."Id" AND i."ReviewStatus" = 1) AS "ImageCount"`,
		linkTable, foreignKey, foreignKey, alias)
}

// ---------------------------------------------------------------------------
// Tags
// ---------------------------------------------------------------------------

// ListTags returns a page of tags with their accepted-image counts.
//
// exposeCreator controls whether CreatorId is echoed back: the C# service nulls
// it for ordinary callers so the list cannot be used to enumerate who submitted
// what. The column is still selected, as a typed NULL, so one scan helper serves
// both cases.
func (r *Repo) ListTags(ctx context.Context, filter CatalogFilter, exposeCreator bool) ([]TagEntry, int64, error) {
	where, args := catalogWhere(filter, "t")

	projection := `SELECT t."Id", t."Name", t."Slug", t."Description", t."CreatorId",
                   t."ReviewStatus", ` + acceptedImageCount("ImageTag", "TagsId", "t")
	if !exposeCreator {
		projection = `SELECT t."Id", t."Name", t."Slug", t."Description", NULL::bigint,
                   t."ReviewStatus", ` + acceptedImageCount("ImageTag", "TagsId", "t")
	}

	// An exact lookup is not paginated: one page holds everything that matched,
	// and the envelope reports pageNumber 1 with totalCount equal to the number
	// returned. Returning a page there would silently hide matches.
	if len(filter.IncludedIDs) > 0 || len(filter.IncludedSlugs) > 0 {
		entries, err := r.queryTagEntries(ctx, projection+`
            FROM "Tags" t WHERE `+where, args...)
		if err != nil {
			return nil, 0, err
		}
		return entries, int64(len(entries)), nil
	}

	var total int64
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM "Tags" t WHERE %s`, where), args...).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count tags: %w", err)
	}

	args = append(args, filter.PageSize, filter.Page*filter.PageSize)
	entries, err := r.queryTagEntries(ctx, fmt.Sprintf(projection+`
            FROM "Tags" t
            WHERE %s
            ORDER BY "ImageCount" DESC
            LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func (r *Repo) queryTagEntries(ctx context.Context, statement string, args ...any) ([]TagEntry, error) {
	rows, err := r.db.Query(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list tags: %w", err)
	}
	defer rows.Close()

	entries := make([]TagEntry, 0, 16)
	for rows.Next() {
		var entry TagEntry
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.Slug, &entry.Description,
			&entry.CreatorID, &entry.ReviewStatus, &entry.ImageCount); err != nil {
			return nil, fmt.Errorf("scan tag: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// GetTagsBySlugs resolves slugs to tags, used when attaching tags to an upload.
//
// An unknown slug fails the whole upload, so this returns the set it found and
// the caller compares lengths rather than silently dropping the missing ones.
//
// Pending tags are included deliberately. UploadImage/Command.cs queries by slug
// with no review-status clause, which is what lets someone attach a tag they
// submitted moments ago to their own upload; filtering to accepted here would
// make that upload fail with a 404.
func (r *Repo) GetTagsBySlugs(ctx context.Context, slugs []string) ([]models.Tag, error) {
	if len(slugs) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"
        FROM "Tags" WHERE "Slug" = ANY($1)`, slugs)
	if err != nil {
		return nil, fmt.Errorf("get tags by slug: %w", err)
	}
	defer rows.Close()

	tags := make([]models.Tag, 0, len(slugs))
	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		tags = append(tags, tag)
	}
	return tags, rows.Err()
}

// GetTagByID fetches a single tag.
func (r *Repo) GetTagByID(ctx context.Context, id int64) (models.Tag, error) {
	return scanTag(r.db.QueryRow(ctx, `
        SELECT "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"
        FROM "Tags" WHERE "Id" = $1`, id))
}

// GetTagBySlug fetches a tag by its slug.
func (r *Repo) GetTagBySlug(ctx context.Context, slug string) (models.Tag, error) {
	return scanTag(r.db.QueryRow(ctx, `
        SELECT "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"
        FROM "Tags" WHERE "Slug" = $1`, slug))
}

// GetTagsByIDs loads several tags at once, keyed by id.
//
// The review queue resolves one target per task, so this replaces a per-task
// lookup with one statement per page.
func (r *Repo) GetTagsByIDs(ctx context.Context, ids []int64) (map[int64]models.Tag, error) {
	out := map[int64]models.Tag{}
	if len(ids) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"
        FROM "Tags" WHERE "Id" = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("get tags by id: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		tag, err := scanTag(rows)
		if err != nil {
			return nil, err
		}
		out[tag.ID] = tag
	}
	return out, rows.Err()
}

// scanTag reads one tag row.
func scanTag(row rowScanner) (models.Tag, error) {
	var tag models.Tag
	err := row.Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description,
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
        INSERT INTO "Tags" ("Name", "Slug", "Description", "CreatorId", "ReviewStatus")
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

// FindTagByNameOrSlug returns an existing tag colliding on slug or name.
//
// The two are reported separately because the frontend shows the message next to
// the field that collided, so "slug already exists" and "name already exists"
// are different responses.
func (r *Repo) FindTagByNameOrSlug(ctx context.Context, name, slug string) (models.Tag, error) {
	return scanTag(r.db.QueryRow(ctx, `
        SELECT "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"
        FROM "Tags"
        WHERE "Slug" = $1 OR LOWER("Name") = LOWER($2)
        LIMIT 1`, slug, name))
}

// TagNameTaken reports whether a different tag already uses this name.
//
// The id is excluded so an edit that keeps its own name is not a conflict.
func (r *Repo) TagNameTaken(ctx context.Context, id int64, name string) (bool, error) {
	var taken bool
	err := r.db.QueryRow(ctx,
		`SELECT EXISTS (SELECT 1 FROM "Tags" WHERE "Id" <> $1 AND LOWER("Name") = LOWER($2))`,
		id, name).Scan(&taken)
	if err != nil {
		return false, fmt.Errorf("check tag name: %w", err)
	}
	return taken, nil
}

// UpdateTag rewrites a tag's editable fields.
//
// An empty slug keeps the existing one rather than clearing it, matching
// UpdateTag/Command.cs, which only touches the slug when a non-empty value is
// supplied.
func (r *Repo) UpdateTag(ctx context.Context, id int64, name, slug, description string) (models.Tag, error) {
	var tag models.Tag
	err := r.db.QueryRow(ctx, `
        UPDATE "Tags"
        SET "Name" = $1,
            "Slug" = COALESCE(NULLIF($2, ''), "Slug"),
            "Description" = $3
        WHERE "Id" = $4
        RETURNING "Id", "Name", "Slug", "Description", "CreatorId", "ReviewStatus"`,
		name, slug, description, id).
		Scan(&tag.ID, &tag.Name, &tag.Slug, &tag.Description,
			&tag.CreatorID, &tag.ReviewStatus)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Tag{}, ErrNotFound
		}
		return models.Tag{}, fmt.Errorf("update tag: %w", err)
	}
	return tag, nil
}

// DeleteTag removes a tag.
//
// There is deliberately no "is this tag in use" guard. The C# service deletes
// unconditionally and lets the ImageTag foreign key cascade, so an admin
// removing a tag accepts that it disappears from every image carrying it.
func (r *Repo) DeleteTag(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM "Tags" WHERE "Id" = $1`, id)
	if err != nil {
		return fmt.Errorf("delete tag: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetTagReviewStatus changes a tag's visibility.
//
// This is the moderator shortcut used from the catalogue rather than the queue;
// the handler closes the open task so the same submission is not shown twice.
func (r *Repo) SetTagReviewStatus(ctx context.Context, id int64, status models.ReviewStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE "Tags" SET "ReviewStatus" = $1 WHERE "Id" = $2`,
		int(status), id)
	if err != nil {
		return fmt.Errorf("set tag review status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetTagCreator reassigns a tag to a different creator.
func (r *Repo) SetTagCreator(ctx context.Context, id int64, creatorID int64) error {
	if _, err := r.db.Exec(ctx, `UPDATE "Tags" SET "CreatorId" = $1 WHERE "Id" = $2`,
		creatorID, id); err != nil {
		return fmt.Errorf("set tag creator: %w", err)
	}
	return nil
}

// ---------------------------------------------------------------------------
// Artists
// ---------------------------------------------------------------------------

// ListArtists returns a page of artists with their accepted-image counts.
func (r *Repo) ListArtists(ctx context.Context, filter CatalogFilter, exposeCreator bool) ([]ArtistEntry, int64, error) {
	where, args := catalogWhere(filter, "a")

	creator := `a."CreatorId"`
	if !exposeCreator {
		creator = `NULL::bigint`
	}
	projection := `SELECT a."Id", a."Name", a."DeviantArt", a."Twitter",
                   a."Patreon", a."Pixiv", ` + creator + `, a."ReviewStatus", ` +
		acceptedImageCount("ArtistImage", "ArtistsId", "a")

	if len(filter.IncludedIDs) > 0 {
		entries, err := r.queryArtistEntries(ctx, projection+`
            FROM "Artists" a WHERE `+where, args...)
		if err != nil {
			return nil, 0, err
		}
		return entries, int64(len(entries)), nil
	}

	var total int64
	if err := r.db.QueryRow(ctx,
		fmt.Sprintf(`SELECT COUNT(*) FROM "Artists" a WHERE %s`, where), args...).
		Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count artists: %w", err)
	}

	args = append(args, filter.PageSize, filter.Page*filter.PageSize)
	entries, err := r.queryArtistEntries(ctx, fmt.Sprintf(projection+`
            FROM "Artists" a
            WHERE %s
            ORDER BY "ImageCount" DESC
            LIMIT $%d OFFSET $%d`, where, len(args)-1, len(args)), args...)
	if err != nil {
		return nil, 0, err
	}
	return entries, total, nil
}

func (r *Repo) queryArtistEntries(ctx context.Context, statement string, args ...any) ([]ArtistEntry, error) {
	rows, err := r.db.Query(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("list artists: %w", err)
	}
	defer rows.Close()

	entries := make([]ArtistEntry, 0, 16)
	for rows.Next() {
		var entry ArtistEntry
		if err := rows.Scan(&entry.ID, &entry.Name, &entry.DeviantArt, &entry.Twitter,
			&entry.Patreon, &entry.Pixiv, &entry.CreatorID, &entry.ReviewStatus,
			&entry.ImageCount); err != nil {
			return nil, fmt.Errorf("scan artist: %w", err)
		}
		entries = append(entries, entry)
	}
	return entries, rows.Err()
}

// GetArtistsByIDs resolves artist IDs, used when attaching artists to an upload.
//
// As with tags, the caller compares lengths to detect unknown IDs, and pending
// artists are included so someone can credit an artist they just submitted.
func (r *Repo) GetArtistsByIDs(ctx context.Context, ids []int64) ([]models.Artist, error) {
	if len(ids) == 0 {
		return nil, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
               "CreatorId", "ReviewStatus"
        FROM "Artists" WHERE "Id" = ANY($1)`, ids)
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
        FROM "Artists" WHERE "Id" = $1`, id))
}

// GetArtistByName fetches an artist by name, case-insensitively.
//
// The frontend supplies whatever the user typed into a creation form, and
// useMetadata looks the entry up before deciding to create a duplicate, so the
// comparison has to tolerate capitalisation.
func (r *Repo) GetArtistByName(ctx context.Context, name string) (models.Artist, error) {
	return scanArtist(r.db.QueryRow(ctx, `
        SELECT "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
               "CreatorId", "ReviewStatus"
        FROM "Artists"
        WHERE LOWER("Name") = LOWER($1)
        ORDER BY "Id" ASC
        LIMIT 1`, name))
}

// scanArtist reads one artist row.
func scanArtist(row rowScanner) (models.Artist, error) {
	var artist models.Artist
	err := row.Scan(&artist.ID, &artist.Name, &artist.DeviantArt, &artist.Twitter,
		&artist.Patreon, &artist.Pixiv, &artist.CreatorID, &artist.ReviewStatus)
	if err != nil {
		return models.Artist{}, fmt.Errorf("scan artist: %w", err)
	}
	return artist, nil
}

// CreateArtist inserts a new artist and returns it.
func (r *Repo) CreateArtist(ctx context.Context, artist models.Artist) (models.Artist, error) {
	return scanArtist(r.db.QueryRow(ctx, `
        INSERT INTO "Artists" ("Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
                              "CreatorId", "ReviewStatus")
        VALUES ($1, $2, $3, $4, $5, $6, $7)
        RETURNING "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
                  "CreatorId", "ReviewStatus"`,
		artist.Name, artist.DeviantArt, artist.Twitter, artist.Patreon, artist.Pixiv,
		artist.CreatorID, int(artist.ReviewStatus)))
}

// FindArtistConflict returns an existing artist colliding on any field.
//
// excludeID is the artist being edited, which must not collide with itself. A
// nil link disables that field's comparison, matching the C# query, which only
// compares the links the caller actually supplied.
func (r *Repo) FindArtistConflict(ctx context.Context, excludeID int64, artist models.Artist) (models.Artist, error) {
	return scanArtist(r.db.QueryRow(ctx, `
        SELECT "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
               "CreatorId", "ReviewStatus"
        FROM "Artists"
        WHERE "Id" <> $1
          AND (LOWER("Name") = LOWER($2)
               OR ($3::text IS NOT NULL AND "Patreon"    = $3)
               OR ($4::text IS NOT NULL AND "Pixiv"      = $4)
               OR ($5::text IS NOT NULL AND "Twitter"    = $5)
               OR ($6::text IS NOT NULL AND "DeviantArt" = $6))
        LIMIT 1`,
		excludeID, artist.Name, artist.Patreon, artist.Pixiv, artist.Twitter, artist.DeviantArt))
}

// UpdateArtist rewrites an artist's details.
func (r *Repo) UpdateArtist(ctx context.Context, artist models.Artist) (models.Artist, error) {
	return scanArtist(r.db.QueryRow(ctx, `
        UPDATE "Artists"
        SET "Name" = $1, "DeviantArt" = $2, "Twitter" = $3, "Patreon" = $4, "Pixiv" = $5
        WHERE "Id" = $6
        RETURNING "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
                  "CreatorId", "ReviewStatus"`,
		artist.Name, artist.DeviantArt, artist.Twitter, artist.Patreon, artist.Pixiv,
		artist.ID))
}

// DeleteArtist removes an artist.
//
// As with tags there is no in-use guard: the ArtistImage foreign key cascades,
// so an admin removing an artist accepts that the credit disappears from every
// image that carried it.
func (r *Repo) DeleteArtist(ctx context.Context, id int64) error {
	tag, err := r.db.Exec(ctx, `DELETE FROM "Artists" WHERE "Id" = $1`, id)
	if err != nil {
		return fmt.Errorf("delete artist: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetArtistReviewStatus changes an artist's visibility.
func (r *Repo) SetArtistReviewStatus(ctx context.Context, id int64, status models.ReviewStatus) error {
	tag, err := r.db.Exec(ctx, `UPDATE "Artists" SET "ReviewStatus" = $1 WHERE "Id" = $2`,
		int(status), id)
	if err != nil {
		return fmt.Errorf("set artist review status: %w", err)
	}
	if tag.RowsAffected() == 0 {
		return ErrNotFound
	}
	return nil
}

// SetArtistCreator reassigns an artist to a different creator.
func (r *Repo) SetArtistCreator(ctx context.Context, id int64, creatorID int64) error {
	if _, err := r.db.Exec(ctx, `UPDATE "Artists" SET "CreatorId" = $1 WHERE "Id" = $2`,
		creatorID, id); err != nil {
		return fmt.Errorf("set artist creator: %w", err)
	}
	return nil
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
        FROM "Users" WHERE "Id" = $1`, id).
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
        FROM "Users" WHERE "DiscordId" = $1`, discordID).
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
// This mirrors LoginWithDiscord/Command.cs, which also creates the user's
// default "Favorites" album on first login; see EnsureDefaultAlbum, which the
// handler calls when the insert reported a new row.
func (r *Repo) UpsertUserByDiscordID(ctx context.Context, discordID, name string, avatarURL *string) (models.User, bool, error) {
	var user models.User
	var inserted bool

	err := r.db.QueryRow(ctx, `
        INSERT INTO "Users" ("DiscordId", "Name", "AvatarUrl", "Role")
        VALUES ($1, $2, $3, 0)
        ON CONFLICT ("DiscordId") DO UPDATE
            SET "Name" = EXCLUDED."Name",
                "AvatarUrl" = EXCLUDED."AvatarUrl"
        RETURNING "Id", "DiscordId", "Name", "AvatarUrl", "Role",
                  "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
                  "IsBlacklisted", "BlacklistReason", (xmax = 0) AS "Inserted"`,
		discordID, name, avatarURL).
		Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL, &user.Role,
			&user.RequestCount, &user.JWTRequestCount, &user.APIKeyRequestCount,
			&user.IsBlacklisted, &user.BlacklistReason, &inserted)
	if err != nil {
		return models.User{}, false, fmt.Errorf("upsert user: %w", err)
	}
	return user, inserted, nil
}

// UpdateUserBan sets or lifts a blacklist.
func (r *Repo) UpdateUserBan(ctx context.Context, userID int64, banned bool, reason *string) (models.User, error) {
	var user models.User
	err := r.db.QueryRow(ctx, `
        UPDATE "Users" SET "IsBlacklisted" = $1, "BlacklistReason" = $2
        WHERE "Id" = $3
        RETURNING "Id", "DiscordId", "Name", "AvatarUrl", "Role",
                  "RequestCount", "JwtRequestCount", "ApiKeyRequestCount",
                  "IsBlacklisted", "BlacklistReason"`, banned, reason, userID).
		Scan(&user.ID, &user.DiscordID, &user.Name, &user.AvatarURL, &user.Role,
			&user.RequestCount, &user.JWTRequestCount, &user.APIKeyRequestCount,
			&user.IsBlacklisted, &user.BlacklistReason)
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.User{}, ErrNotFound
		}
		return models.User{}, fmt.Errorf("update user ban: %w", err)
	}
	return user, nil
}

// ---------------------------------------------------------------------------
// Banner
// ---------------------------------------------------------------------------

// BannerSingletonID is the one row the banner lives in.
//
// SiteBanner is a single-row table: the banner is addressed by a constant id, not
// by whichever row happens to exist first.
const BannerSingletonID = 1

// bannerColumns is the projection for the banner row.
const bannerColumns = `"Id", "Message", "Variant", "IsEnabled", "UpdatedAt", "UpdatedById"`

func scanBanner(row rowScanner) (models.SiteBanner, error) {
	var banner models.SiteBanner
	if err := row.Scan(&banner.ID, &banner.Message, &banner.Variant,
		&banner.IsEnabled, &banner.UpdatedAt, &banner.UpdatedByID); err != nil {
		return models.SiteBanner{}, err
	}
	return banner, nil
}

// GetActiveBanner returns the enabled banner, or nil when it is switched off.
//
// A disabled banner is indistinguishable from an absent one to the caller, which
// is why both produce a nil and the handler answers 204 for each.
func (r *Repo) GetActiveBanner(ctx context.Context) (*models.SiteBanner, error) {
	banner, err := scanBanner(r.db.QueryRow(ctx,
		`SELECT `+bannerColumns+`
         FROM "SiteBanners"
         WHERE "Id" = $1 AND "IsEnabled" = true`, BannerSingletonID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return nil, nil
		}
		return nil, fmt.Errorf("get active banner: %w", err)
	}
	return &banner, nil
}

// GetBannerSettings returns the banner row plus its editor, regardless of
// enabled state.
//
// A missing row is not an error: it means the banner has never been configured,
// and the handler answers with defaults so the settings page still renders.
func (r *Repo) GetBannerSettings(ctx context.Context) (models.SiteBanner, *models.User, error) {
	banner, err := scanBanner(r.db.QueryRow(ctx,
		`SELECT `+bannerColumns+` FROM "SiteBanners" WHERE "Id" = $1`, BannerSingletonID))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.SiteBanner{}, nil, nil
		}
		return models.SiteBanner{}, nil, fmt.Errorf("get banner settings: %w", err)
	}

	if banner.UpdatedByID == nil {
		return banner, nil, nil
	}
	editor, err := r.GetUserByID(ctx, *banner.UpdatedByID)
	if err != nil {
		// A banner whose editor has since been removed is still readable; only
		// the attribution is lost.
		return banner, nil, nil
	}
	return banner, &editor, nil
}

// SaveBanner creates or replaces the singleton banner row and returns the stored
// state with its editor.
//
// The insert carries the singleton id explicitly rather than relying on the
// identity column, so a row created before the table was constrained is adopted
// rather than duplicated.
func (r *Repo) SaveBanner(ctx context.Context, message string, variant models.BannerVariant, enabled bool, editorID *int64) (models.SiteBanner, *models.User, error) {
	var banner models.SiteBanner
	err := r.db.QueryRow(ctx, `
        INSERT INTO "SiteBanners" ("Id", "Message", "Variant", "IsEnabled", "UpdatedAt", "UpdatedById")
        VALUES ($1, $2, $3, $4, now(), $5)
        ON CONFLICT ("Id") DO UPDATE
            SET "Message" = EXCLUDED."Message",
                "Variant" = EXCLUDED."Variant",
                "IsEnabled" = EXCLUDED."IsEnabled",
                "UpdatedAt" = now(),
                "UpdatedById" = EXCLUDED."UpdatedById"
        RETURNING `+bannerColumns,
		BannerSingletonID, message, int(variant), enabled, editorID).
		Scan(&banner.ID, &banner.Message, &banner.Variant,
			&banner.IsEnabled, &banner.UpdatedAt, &banner.UpdatedByID)
	if err != nil {
		return models.SiteBanner{}, nil, fmt.Errorf("save banner: %w", err)
	}

	if editorID == nil {
		return banner, nil, nil
	}
	editor, err := r.GetUserByID(ctx, *editorID)
	if err != nil {
		return banner, nil, nil
	}
	return banner, &editor, nil
}

// ---------------------------------------------------------------------------
// Stats
// ---------------------------------------------------------------------------

// GetPublicStats returns the counters shown on the home page.
//
// Every count excludes pending content, so the public figures only ever describe
// what an anonymous visitor can already see.
func (r *Repo) GetPublicStats(ctx context.Context) (models.PublicStats, error) {
	var stats models.PublicStats

	err := r.db.QueryRow(ctx, `
        SELECT
            (SELECT COALESCE(SUM("RequestCount"), 0) FROM "DailyStats"),
            (SELECT COUNT(*) FROM "Images" WHERE "ReviewStatus" = $1),
            (SELECT COUNT(*) FROM "Tags"   WHERE "ReviewStatus" = $1),
            (SELECT COUNT(*) FROM "Artists" WHERE "ReviewStatus" = $1)`,
		int(models.ReviewAccepted)).
		Scan(&stats.TotalRequests, &stats.TotalImages, &stats.TotalTags, &stats.TotalArtists)
	if err != nil {
		return models.PublicStats{}, fmt.Errorf("get public stats: %w", err)
	}
	return stats, nil
}

// GetArtistsByIDsMap loads several artists at once, keyed by id.
//
// This is the map-returning twin of GetArtistsByIDs: the review queue needs
// random access by target id, while the upload path wants an ordered slice it
// can match positionally against the submitted ids.
func (r *Repo) GetArtistsByIDsMap(ctx context.Context, ids []int64) (map[int64]models.Artist, error) {
	out := map[int64]models.Artist{}
	if len(ids) == 0 {
		return out, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT "Id", "Name", "DeviantArt", "Twitter", "Patreon", "Pixiv",
               "CreatorId", "ReviewStatus"
        FROM "Artists" WHERE "Id" = ANY($1)`, ids)
	if err != nil {
		return nil, fmt.Errorf("get artists by id: %w", err)
	}
	defer rows.Close()

	for rows.Next() {
		artist, err := scanArtist(rows)
		if err != nil {
			return nil, err
		}
		out[artist.ID] = artist
	}
	return out, rows.Err()
}
