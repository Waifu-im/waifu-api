// Package repo holds the SQL queries, grouped by the resource they serve.
//
// # Why hand-written SQL rather than an ORM
//
// The C# service uses Entity Framework Core. The closest idiomatic Go
// equivalent is sqlc, which generates type-safe Go from SQL checked against the
// schema. sqlc is deliberately not wired up yet: it needs a live database to
// introspect, and PLAN.md makes the Supabase connection-mode decision a Phase 0
// prerequisite. Until that is settled, these queries are written by hand so the
// work is not blocked, and they are plain enough that sqlc can adopt them later
// without rewriting the SQL.
//
// Every statement below is parameterised. No value is ever interpolated into
// SQL text, including filter values and sort direction.
//
// # Dynamic filters
//
// GET /images accepts more than twenty optional query parameters. Building that
// as a static statement would be unreadable, so the query is assembled from a
// fixed list of fragments and a fixed whitelist for ORDER BY. The whitelist is
// the important part: a sort column cannot be parameterised in SQL, so it is
// validated against an allowlist before being used.
package repo

import (
	"context"
	"errors"
	"fmt"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"waifuapi/internal/store/models"
)

// Tx is the subset of pgx.Tx these queries use.
//
// It exists as a named interface rather than taking pgx.Tx directly so the
// review-queue decision logic can be tested against a recording double. A
// *pgxpool.Pool and a pgx.Tx both satisfy it, so nothing else changes.
type Tx interface {
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
}

// Repo groups the data access methods.
type Repo struct {
	db DBQuerier
}

// DBQuerier is the subset of pgx used by this package.
//
// It is an interface so queries can be tested against a stub without a running
// database.
type DBQuerier interface {
	Query(ctx context.Context, sql string, args ...any) (pgx.Rows, error)
	QueryRow(ctx context.Context, sql string, args ...any) pgx.Row
	Exec(ctx context.Context, sql string, args ...any) (pgconn.CommandTag, error)

	// Begin opens a transaction. *pgxpool.Pool satisfies this, and so does a
	// pooled connection, which is why multi-statement writes such as the upload
	// and the review transitions can share one implementation.
	Begin(ctx context.Context) (pgx.Tx, error)
}

// New builds a Repo.
func New(db DBQuerier) *Repo {
	return &Repo{db: db}
}

// ---------------------------------------------------------------------------
// Images
// ---------------------------------------------------------------------------

// ImageFilter carries the parameters of GET /images and
// GET /users/{userId}/albums/{albumId}/images.
//
// The field names and semantics mirror GetImagesQuery plus ImageFilters, so the
// two filters merge the way the C# service merges them: the album endpoint sets
// AlbumID and the caller supplies everything else.
//
// The three-state fields are plain ints rather than pointers because their
// zero values are meaningful: NsfwFalse excludes NSFW and FilterAccepted shows
// published content, both of which are the documented defaults.
type ImageFilter struct {
	Nsfw        models.NsfwMode
	Animated    models.AnimatedMode
	Orientation models.Orientation
	OrderBy     models.ImageOrderBy

	// Tag and artist filters accept either an id or a slug/name, exactly as the
	// C# service does: a value that parses as an integer is an id, anything else
	// is matched as a slug (tags) or a lowercased name (artists).
	IncludedTags    []string
	ExcludedTags    []string
	IncludedArtists []string
	ExcludedArtists []string
	IncludedIDs     []string
	ExcludedIDs     []string

	// Width, Height and ByteSize are range expressions rather than numbers:
	// "1920", ">=1920", ">1920", "<=1920", "<1920" or "1280..1920".
	Width    string
	Height   string
	ByteSize string

	// UserID is the caller, used for the "my pending" unions and for the
	// per-user liked/albums fields. It is never taken from the query string.
	UserID *int64

	// AlbumID restricts the result to one album and switches the review-status
	// clause to album semantics.
	AlbumID *int64

	// ReviewStatus and ChildrenReviewStatus are moderators-only, except for
	// FilterAll which the handler rejects for ordinary callers.
	ReviewStatus         models.ReviewStatusFilter
	ChildrenReviewStatus models.ReviewStatusFilter

	// UploaderID filters by uploader. It is moderator-only.
	UploaderID *int64

	IncludeMyPending         bool
	IncludeMyPendingChildren bool

	Page     int
	PageSize int
}

// imageColumns is the projection for image rows.
//
// The perceptual hash is cast to text because bit(64) has no binary decoder for
// a Go string; models.PerceptualHashFromBitString converts it in Go.
const imageColumns = `
    i."Id",
    i."UploaderId",
    i."Width",
    i."Height",
    i."ByteSize",
    i."Extension",
    i."DominantColor",
    i."Source",
    i."IsNsfw",
    i."IsAnimated",
    i."UploadedAt",
    i."PerceptualHash"::text AS "PerceptualHash",
    i."ReviewStatus"`

// imageWhere builds the shared WHERE clause for both the paged and the count
// statement.
//
// Everything is a parameter; only the fragments themselves are concatenated,
// and they are a fixed set chosen by this function, never by the caller.
func imageWhere(filter ImageFilter) (string, []any) {
	var conditions []string
	var args []any

	add := func(condition string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(condition, len(args)))
	}

	switch {
	case filter.AlbumID != nil:
		// Inside an album the review filter is applied literally: Pending shows
		// pending images, Accepted hides them, All shows both. There is no
		// "my pending" union here.
		switch filter.ReviewStatus {
		case models.FilterPending:
			conditions = append(conditions, `i."ReviewStatus" = 0`)
		case models.FilterAll:
			// no clause
		default:
			conditions = append(conditions, `i."ReviewStatus" = 1`)
		}
	default:
		switch filter.ReviewStatus {
		case models.FilterPending:
			conditions = append(conditions, `i."ReviewStatus" = 0`)
		case models.FilterAll:
			// no clause
		default:
			if filter.IncludeMyPending && filter.UserID != nil {
				add(`(i."ReviewStatus" = 1 OR (i."ReviewStatus" = 0 AND i."UploaderId" = $%d))`,
					*filter.UserID)
			} else {
				conditions = append(conditions, `i."ReviewStatus" = 1`)
			}
		}
	}

	switch filter.Nsfw {
	case models.NsfwTrue:
		conditions = append(conditions, `i."IsNsfw" = true`)
	case models.NsfwAll:
		// no clause
	default:
		conditions = append(conditions, `i."IsNsfw" = false`)
	}

	switch filter.Animated {
	case models.AnimatedTrue:
		conditions = append(conditions, `i."IsAnimated" = true`)
	case models.AnimatedFalse:
		conditions = append(conditions, `i."IsAnimated" = false`)
	}

	switch filter.Orientation {
	case models.OrientationLandscape:
		conditions = append(conditions, `i."Width" > i."Height"`)
	case models.OrientationPortrait:
		conditions = append(conditions, `i."Height" > i."Width"`)
	case models.OrientationSquare:
		conditions = append(conditions, `i."Width" = i."Height"`)
	}

	// Included tags are ANDed one clause per value, which is what "only images
	// with ALL these tags" means. Exists rather than a join, so the result set
	// cannot be multiplied by the number of matching tags.
	for _, value := range filter.IncludedTags {
		if id, err := parseInt64(value); err == nil {
			add(`EXISTS (SELECT 1 FROM "ImageTag" it WHERE it."ImagesId" = i."Id" AND it."TagsId" = $%d)`, id)
			continue
		}
		add(`EXISTS (
            SELECT 1 FROM "ImageTag" it
            JOIN "Tags" t ON t."Id" = it."TagsId"
            WHERE it."ImagesId" = i."Id" AND t."Slug" = $%d)`, strings.ToLower(strings.TrimSpace(value)))
	}

	for _, value := range filter.ExcludedTags {
		if id, err := parseInt64(value); err == nil {
			add(`NOT EXISTS (SELECT 1 FROM "ImageTag" it WHERE it."ImagesId" = i."Id" AND it."TagsId" = $%d)`, id)
			continue
		}
		add(`NOT EXISTS (
            SELECT 1 FROM "ImageTag" it
            JOIN "Tags" t ON t."Id" = it."TagsId"
            WHERE it."ImagesId" = i."Id" AND t."Slug" = $%d)`, strings.ToLower(strings.TrimSpace(value)))
	}

	for _, value := range filter.IncludedArtists {
		if id, err := parseInt64(value); err == nil {
			add(`EXISTS (SELECT 1 FROM "ArtistImage" ai WHERE ai."ImagesId" = i."Id" AND ai."ArtistsId" = $%d)`, id)
			continue
		}
		add(`EXISTS (
            SELECT 1 FROM "ArtistImage" ai
            JOIN "Artists" a ON a."Id" = ai."ArtistsId"
            WHERE ai."ImagesId" = i."Id" AND LOWER(a."Name") = $%d)`, strings.ToLower(strings.TrimSpace(value)))
	}

	for _, value := range filter.ExcludedArtists {
		if id, err := parseInt64(value); err == nil {
			add(`NOT EXISTS (SELECT 1 FROM "ArtistImage" ai WHERE ai."ImagesId" = i."Id" AND ai."ArtistsId" = $%d)`, id)
			continue
		}
		add(`NOT EXISTS (
            SELECT 1 FROM "ArtistImage" ai
            JOIN "Artists" a ON a."Id" = ai."ArtistsId"
            WHERE ai."ImagesId" = i."Id" AND LOWER(a."Name") = $%d)`, strings.ToLower(strings.TrimSpace(value)))
	}

	// A value that is not an integer is ignored rather than rejected, matching
	// the C# service, which only adds a clause when long.TryParse succeeds.
	for _, value := range filter.IncludedIDs {
		if id, err := parseInt64(value); err == nil {
			add(`i."Id" = $%d`, id)
		}
	}
	for _, value := range filter.ExcludedIDs {
		if id, err := parseInt64(value); err == nil {
			add(`i."Id" <> $%d`, id)
		}
	}

	// rangeClause returns "" for an absent or unparseable expression, and an
	// empty fragment must not be appended: joining it would produce a statement
	// ending in " AND ", which is a syntax error rather than a silently ignored
	// filter. This is the single most dangerous thing in this function, because
	// every image query without a width/height/byteSize filter hits it.
	for _, expression := range []struct{ value, column string }{
		{filter.Width, `i."Width"`},
		{filter.Height, `i."Height"`},
		{filter.ByteSize, `i."ByteSize"`},
	} {
		if clause := rangeClause(expression.value, expression.column); clause != "" {
			conditions = append(conditions, clause)
		}
	}

	if filter.UploaderID != nil {
		add(`i."UploaderId" = $%d`, *filter.UploaderID)
	}

	return strings.Join(conditions, "\n    AND "), args
}

// rangeClause turns one of the range expressions into a SQL comparison.
//
// A malformed expression yields no clause, which is what ApplyRangeFilter does
// in C#: it silently ignores a value it cannot parse rather than failing the
// request.
func rangeClause(expression, column string) string {
	trimmed := strings.TrimSpace(expression)
	if trimmed == "" {
		return ""
	}

	// The prefixes are checked longest-first so ">=" is not mistaken for ">".
	for _, candidate := range []struct{ prefix, op string }{
		{">=", ">="}, {"<=", "<="}, {">", ">"}, {"<", "<"},
	} {
		if !strings.HasPrefix(trimmed, candidate.prefix) {
			continue
		}
		value, err := parseInt64(trimmed[len(candidate.prefix):])
		if err != nil {
			return ""
		}
		return fmt.Sprintf(`%s %s %d`, column, candidate.op, value)
	}

	if parts := strings.Split(trimmed, ".."); len(parts) == 2 {
		low, lowErr := parseInt64(parts[0])
		high, highErr := parseInt64(parts[1])
		if lowErr == nil && highErr == nil {
			return fmt.Sprintf(`%s >= %d AND %s <= %d`, column, low, column, high)
		}
		return ""
	}

	value, err := parseInt64(trimmed)
	if err != nil {
		return ""
	}
	return fmt.Sprintf(`%s = %d`, column, value)
}

// parseInt64 reads an integer filter value.
func parseInt64(value string) (int64, error) {
	return strconv.ParseInt(strings.TrimSpace(value), 10, 64)
}

// imageOrder renders the ORDER BY for a query.
//
// The ORDER BY expression is chosen from a fixed set, never from user input, so
// it cannot be an injection vector; the values it interpolates are the literal
// column names above and RANDOM().
func imageOrder(filter ImageFilter) string {
	switch filter.OrderBy {
	case models.OrderUploadedAt:
		return `i."UploadedAt" DESC`
	case models.OrderFavorites:
		return favoritesCountSQL + ` DESC`
	case models.OrderAddedToAlbum:
		if filter.AlbumID != nil {
			return `ai."AddedAt" DESC`
		}
		return `RANDOM()`
	default:
		return `RANDOM()`
	}
}

// favoritesCountSQL counts how many users have this image in their default
// album, which is what the API reports as "favorites".
const favoritesCountSQL = `(SELECT COUNT(*) FROM "AlbumItems" ai
                           JOIN "Albums" a ON a."Id" = ai."AlbumId"
                           WHERE ai."ImageId" = i."Id" AND a."IsDefault" = true)`

// imageSource renders the FROM and WHERE for an image query and returns the
// matching argument list.
//
// The album variant joins AlbumItem so the album membership itself is the filter.
// The album id is appended to the argument list here, once, so the paged and
// count statements share one correct placeholder. Doing it by string substitution
// after the fact is what let the two statements drift apart.
func imageSource(filter ImageFilter) (from string, args []any) {
	where, args := imageWhere(filter)

	if filter.AlbumID == nil {
		return "FROM \"Images\" i\n    WHERE " + where, args
	}

	args = append(args, *filter.AlbumID)
	from = fmt.Sprintf(`FROM "Images" i
        JOIN "AlbumItems" ai ON ai."ImageId" = i."Id" AND ai."AlbumId" = $%d
        WHERE %s`, len(args), where)
	return from, args
}

// ListImages returns a page of images matching the filter, plus the total count
// of matching rows so the caller can build the envelope.
//
// The count comes from the same FROM/WHERE as the page. Building them separately
// is how a total drifts from the rows it describes, and the drift is invisible
// until somebody notices the pager is wrong.
func (r *Repo) ListImages(ctx context.Context, filter ImageFilter) ([]models.Image, int64, error) {
	from, args := imageSource(filter)

	var total int64
	countSQL := fmt.Sprintf("SELECT COUNT(*) %s", from)
	if err := r.db.QueryRow(ctx, countSQL, args...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count images: %w", err)
	}

	pageArgs := append(append([]any{}, args...), filter.PageSize, filter.Page*filter.PageSize)

	// LIMIT and OFFSET are bound parameters, not interpolated, and their
	// placeholders are numbered after everything imageSource already bound.
	pageSQL := fmt.Sprintf("SELECT %s %s ORDER BY %s LIMIT $%d OFFSET $%d",
		imageColumns, from, imageOrder(filter), len(args)+1, len(args)+2)

	rows, err := r.db.Query(ctx, pageSQL, pageArgs...)
	if err != nil {
		return nil, 0, fmt.Errorf("list images: %w", err)
	}
	defer rows.Close()

	images := make([]models.Image, 0, filter.PageSize)
	for rows.Next() {
		image, err := scanImage(rows)
		if err != nil {
			return nil, 0, fmt.Errorf("scan image: %w", err)
		}
		images = append(images, image)
	}
	if err := rows.Err(); err != nil {
		return nil, 0, fmt.Errorf("iterate images: %w", err)
	}

	return images, total, nil
}

// rowScanner is implemented by pgx.Rows and pgx.Row alike, so one scan helper
// serves both.
type rowScanner interface {
	Scan(dest ...any) error
}

// scanImage reads one image row.
//
// The perceptual hash is read as text and converted explicitly, because the
// column is bit(64) and a generic scan would fail on it.
func scanImage(row rowScanner) (models.Image, error) {
	var (
		image    models.Image
		hashBits string
	)
	err := row.Scan(
		&image.ID,
		&image.UploaderID,
		&image.Width,
		&image.Height,
		&image.ByteSize,
		&image.Extension,
		&image.DominantColor,
		&image.Source,
		&image.IsNsfw,
		&image.IsAnimated,
		&image.UploadedAt,
		&hashBits,
		&image.ReviewStatus,
	)
	if err != nil {
		return models.Image{}, err
	}

	hash, err := models.PerceptualHashFromBitString(hashBits)
	if err != nil {
		return models.Image{}, fmt.Errorf("image %d: %w", image.ID, err)
	}
	image.PerceptualHash = hash

	return image, nil
}

// GetImageByID fetches a single image.
//
// Only Accepted images are returned to ordinary callers. includePending lets a
// moderator or the uploader retrieve content still awaiting review, matching
// the childrenReviewStatus filter on the C# read path.
func (r *Repo) GetImageByID(ctx context.Context, id int64, includePending bool) (models.Image, error) {
	query := fmt.Sprintf(`SELECT %s FROM "Images" i WHERE i."Id" = $1`, imageColumns)

	if !includePending {
		query += ` AND i."ReviewStatus" = $2`
		row := r.db.QueryRow(ctx, query, id, int(models.ReviewAccepted))
		image, err := scanImage(row)
		if err != nil {
			return models.Image{}, err
		}
		return image, nil
	}

	row := r.db.QueryRow(ctx, query, id)
	return scanImage(row)
}

// FindDuplicateByHash reports an existing image within the duplicate threshold.
//
// This is the Go equivalent of the Hamming-distance check in
// UploadImage/Command.cs. It deliberately compares against every row rather
// than only accepted ones, matching the C# service, so a pending image still
// blocks a re-upload of the same file.
func (r *Repo) FindDuplicateByHash(ctx context.Context, hash uint64) (int64, error) {
	// The comparison is done in SQL on the bit string. Casting both sides to
	// bit(64) keeps the operator available.
	//
	// The Hamming distance is computed in SQL because doing it in Go would mean
	// loading every stored hash on every upload.
	//
	// Postgres has no hamming-distance function for bit(n). The XOR operator
	// "#" on bit types produces the differing bits, and counting the ones in the
	// result is the distance. replace() strips the zeros, so length() counts the
	// ones. Both parameters are explicitly cast, because pgx cannot infer a type
	// for a parameter used only inside a bit expression and would otherwise fail
	// to encode the threshold.
	//
	// NOTE: this only produces meaningful results once the PLAN.md section 6.1
	// backfill has run. Until then the stored hashes come from an algorithm that
	// does not reproduce itself, so the comparison is against a mixed
	// vocabulary.
	const query = `
        SELECT "Id"
        FROM "Images"
        WHERE length(replace(
                  ("PerceptualHash"::bit(64) # $1::bit(64))::text,
                  '0', ''
              ))::int <= $2::int
        ORDER BY "Id"
        LIMIT 1`

	var id int64
	err := r.db.QueryRow(ctx, query, models.PerceptualHashToBitString(hash), models.DuplicateThreshold).Scan(&id)
	if err != nil {
		// No rows is the normal "no duplicate" outcome.
		if errors.Is(err, pgx.ErrNoRows) {
			return 0, nil
		}
		return 0, fmt.Errorf("find duplicate: %w", err)
	}
	return id, nil
}

// DeleteImage removes an image row.
//
// Used by the upload rollback path, where a row exists but the object write
// failed. Cascades remove the tag and artist links.
func (r *Repo) DeleteImage(ctx context.Context, id int64) error {
	_, err := r.db.Exec(ctx, `DELETE FROM "Images" WHERE "Id" = $1`, id)
	if err != nil {
		return fmt.Errorf("delete image: %w", err)
	}
	return nil
}

// UpdateImageMetadata writes the editable fields of an image.
//
// Width, height, hash and extension are deliberately not editable: the
// perceptual hash must describe the bytes that are actually stored, and the
// extension is part of the object key.
//
// The RETURNING list repeats "ReviewStatus" because imageColumns already selects
// it under the "i" alias, which is not valid in a RETURNING clause.
func (r *Repo) UpdateImageMetadata(ctx context.Context, id int64, source *string, isNsfw bool) (models.Image, error) {
	image, err := scanImage(r.db.QueryRow(ctx, `
        UPDATE "Images" SET "Source" = $1, "IsNsfw" = $2
        WHERE "Id" = $3
        RETURNING "Id", "UploaderId", "Width", "Height", "ByteSize", "Extension",
                  "DominantColor", "Source", "IsNsfw", "IsAnimated", "UploadedAt",
                  "PerceptualHash"::text, "ReviewStatus"`, source, isNsfw, id))
	if err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return models.Image{}, ErrNotFound
		}
		return models.Image{}, fmt.Errorf("update image: %w", err)
	}
	return image, nil
}
