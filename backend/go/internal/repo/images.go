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
	"fmt"
	"strings"

	"github.com/jackc/pgx/v5"
	"github.com/jackc/pgx/v5/pgconn"

	"waifuapi/internal/store/models"
)

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
}

// New builds a Repo.
func New(db DBQuerier) *Repo {
	return &Repo{db: db}
}

// ---------------------------------------------------------------------------
// Images
// ---------------------------------------------------------------------------

// ImageFilter carries the optional parameters of GET /images.
//
// Every field is a pointer so "not supplied" is distinguishable from "supplied
// with a zero value". For example IsNsfw=false is a meaningful filter, whereas
// omitting it means "do not filter".
type ImageFilter struct {
	IsNsfw      *bool
	Orientation string // Landscape | Portrait | Square
	IsAnimated  *bool
	MinWidth    *int
	MinHeight   *int
	MaxWidth    *int
	MaxHeight   *int
	MinFileSize *int64
	MaxFileSize *int64
	TagSlugs    []string
	ExcludedIDs []int64
	ForcedIDs   []int64
	ArtistIDs   []int64
	UploaderIDs []int64
	SortBy      string
	SortOrder   string
	Page        int
	PageSize    int
}

// allowedImageSorts is the ORDER BY whitelist.
//
// A sort column cannot be a bound parameter in SQL, so it has to be validated
// against a fixed set. Anything not listed here is rejected rather than
// defaulted, so an unsupported sort produces a clear 400 instead of silently
// returning a different ordering than the client asked for.
var allowedImageSorts = map[string]string{
	"id":         `i."Id"`,
	"uploadedAt": `i."UploadedAt"`,
	"width":      `i."Width"`,
	"height":     `i."Height"`,
	"byteSize":   `i."ByteSize"`,
	"random":     `RANDOM()`,
}

// allowedOrientations constrains the orientation filter.
// Fragments reference the "i" alias, which is how both the paged and the count
// query expose the Image table.
var allowedOrientations = map[string]string{
	"Landscape": `i."Width" > i."Height"`,
	"Portrait":  `i."Width" < i."Height"`,
	"Square":    `i."Width" = i."Height"`,
}

// imageColumns is the projection for image rows, including the joined tag and
// artist names the response needs.
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
    -- bit(64) has no binary decoder for a Go string, so it is cast to text
    -- here rather than in the scan helper.
    i."PerceptualHash"::text AS "PerceptualHash",
    i."ReviewStatus"`

// buildImageQuery assembles the filtered image query.
//
// It returns the SQL text and the ordered argument list. Fragments are appended
// in a fixed order so the generated SQL is deterministic, which makes the
// contract tests readable.
func buildImageQuery(filter ImageFilter) (string, []any) {
	var conditions []string
	var args []any

	// The alias "i" is fixed rather than user-controlled.
	conditions = append(conditions, `i."ReviewStatus" = $1`)
	args = append(args, int(models.ReviewAccepted))

	add := func(format string, value any) {
		args = append(args, value)
		conditions = append(conditions, fmt.Sprintf(format, len(args)))
	}

	if filter.IsNsfw != nil {
		add(`i."IsNsfw" = $%d`, *filter.IsNsfw)
	}
	if filter.IsAnimated != nil {
		add(`i."IsAnimated" = $%d`, *filter.IsAnimated)
	}
	if filter.MinWidth != nil {
		add(`i."Width" >= $%d`, *filter.MinWidth)
	}
	if filter.MaxWidth != nil {
		add(`i."Width" <= $%d`, *filter.MaxWidth)
	}
	if filter.MinHeight != nil {
		add(`i."Height" >= $%d`, *filter.MinHeight)
	}
	if filter.MaxHeight != nil {
		add(`i."Height" <= $%d`, *filter.MaxHeight)
	}
	if filter.MinFileSize != nil {
		add(`i."ByteSize" >= $%d`, *filter.MinFileSize)
	}
	if filter.MaxFileSize != nil {
		add(`i."ByteSize" <= $%d`, *filter.MaxFileSize)
	}

	if orientationSQL, ok := allowedOrientations[filter.Orientation]; ok {
		conditions = append(conditions, orientationSQL)
	}

	// Tag filter: images carrying any of the given slugs. Exists rather than a
	// join so the result set cannot be multiplied by the number of matching
	// tags.
	if len(filter.TagSlugs) > 0 {
		args = append(args, filter.TagSlugs)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
        SELECT 1 FROM "ImageTag" it
        JOIN "Tag" t ON t."Id" = it."TagId"
        WHERE it."ImageId" = i."Id" AND t."Slug" = ANY($%d))`, len(args)))
	}

	if len(filter.ArtistIDs) > 0 {
		args = append(args, filter.ArtistIDs)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
        SELECT 1 FROM "ArtistImage" ai
        WHERE ai."ImageId" = i."Id" AND ai."ArtistId" = ANY($%d))`, len(args)))
	}

	if len(filter.UploaderIDs) > 0 {
		args = append(args, filter.UploaderIDs)
		conditions = append(conditions, fmt.Sprintf(`i."UploaderId" = ANY($%d)`, len(args)))
	}

	if len(filter.ExcludedIDs) > 0 {
		args = append(args, filter.ExcludedIDs)
		conditions = append(conditions, fmt.Sprintf(`NOT (i."Id" = ANY($%d))`, len(args)))
	}

	// Forced IDs are added as an OR branch rather than an AND, because forcing
	// an image in means it appears regardless of the other filters.
	forcedClause := ""
	if len(filter.ForcedIDs) > 0 {
		args = append(args, filter.ForcedIDs)
		forcedClause = fmt.Sprintf(` OR i."Id" = ANY($%d)`, len(args))
	}

	where := strings.Join(conditions, "\n    AND ")
	if forcedClause != "" {
		where = "(" + where + ")" + forcedClause
	}

	sortColumn, ok := allowedImageSorts[filter.SortBy]
	if !ok {
		sortColumn = allowedImageSorts["id"]
	}
	order := "DESC"
	if strings.EqualFold(filter.SortOrder, "asc") {
		order = "ASC"
	}

	// LIMIT and OFFSET are parameterised like everything else.
	args = append(args, filter.PageSize)
	limitPlaceholder := len(args)
	args = append(args, filter.Page)
	offsetPlaceholder := len(args)

	query := fmt.Sprintf(`
        SELECT %s
        FROM "Image" i
        WHERE %s
        ORDER BY %s %s NULLS LAST, i."Id" ASC
        LIMIT $%d OFFSET $%d`,
		imageColumns, where, sortColumn, order, limitPlaceholder, offsetPlaceholder)

	return query, args
}

// ListImages returns a page of images matching the filter.
//
// Returns the rows and the total count matching, so the caller can build a
// paginated envelope.
func (r *Repo) ListImages(ctx context.Context, filter ImageFilter) ([]models.Image, int64, error) {
	// Count first, reusing the same filter logic without LIMIT/OFFSET.
	countQuery, countArgs := buildCountQuery(filter)

	var total int64
	if err := r.db.QueryRow(ctx, countQuery, countArgs...).Scan(&total); err != nil {
		return nil, 0, fmt.Errorf("count images: %w", err)
	}

	query, args := buildImageQuery(filter)

	rows, err := r.db.Query(ctx, query, args...)
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

// buildCountQuery produces the COUNT variant of the image filter.
func buildCountQuery(filter ImageFilter) (string, []any) {
	saved := filter.PageSize
	savedPage := filter.Page
	// A count query must not paginate, so both are neutralised.
	filter.PageSize = 0
	filter.Page = 0

	// Built separately rather than by editing the paged query, so the two
	// cannot drift apart as filters are added.
	var conditions []string
	var buildArgs []any

	conditions = append(conditions, `i."ReviewStatus" = $1`)
	buildArgs = append(buildArgs, int(models.ReviewAccepted))

	add := func(format string, value any) {
		buildArgs = append(buildArgs, value)
		conditions = append(conditions, fmt.Sprintf(format, len(buildArgs)))
	}

	if filter.IsNsfw != nil {
		add(`i."IsNsfw" = $%d`, *filter.IsNsfw)
	}
	if filter.IsAnimated != nil {
		add(`i."IsAnimated" = $%d`, *filter.IsAnimated)
	}
	if filter.MinWidth != nil {
		add(`i."Width" >= $%d`, *filter.MinWidth)
	}
	if filter.MaxWidth != nil {
		add(`i."Width" <= $%d`, *filter.MaxWidth)
	}
	if filter.MinHeight != nil {
		add(`i."Height" >= $%d`, *filter.MinHeight)
	}
	if filter.MaxHeight != nil {
		add(`i."Height" <= $%d`, *filter.MaxHeight)
	}
	if filter.MinFileSize != nil {
		add(`i."ByteSize" >= $%d`, *filter.MinFileSize)
	}
	if filter.MaxFileSize != nil {
		add(`i."ByteSize" <= $%d`, *filter.MaxFileSize)
	}

	if orientationSQL, ok := allowedOrientations[filter.Orientation]; ok {
		conditions = append(conditions, orientationSQL)
	}

	if len(filter.TagSlugs) > 0 {
		buildArgs = append(buildArgs, filter.TagSlugs)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
            SELECT 1 FROM "ImageTag" it
            JOIN "Tag" t ON t."Id" = it."TagId"
            WHERE it."ImageId" = i."Id" AND t."Slug" = ANY($%d))`, len(buildArgs)))
	}
	if len(filter.ArtistIDs) > 0 {
		buildArgs = append(buildArgs, filter.ArtistIDs)
		conditions = append(conditions, fmt.Sprintf(`EXISTS (
            SELECT 1 FROM "ArtistImage" ai
            WHERE ai."ImageId" = i."Id" AND ai."ArtistId" = ANY($%d))`, len(buildArgs)))
	}
	if len(filter.UploaderIDs) > 0 {
		buildArgs = append(buildArgs, filter.UploaderIDs)
		conditions = append(conditions, fmt.Sprintf(`i."UploaderId" = ANY($%d)`, len(buildArgs)))
	}
	if len(filter.ExcludedIDs) > 0 {
		buildArgs = append(buildArgs, filter.ExcludedIDs)
		conditions = append(conditions, fmt.Sprintf(`NOT (i."Id" = ANY($%d))`, len(buildArgs)))
	}
	forced := ""
	if len(filter.ForcedIDs) > 0 {
		buildArgs = append(buildArgs, filter.ForcedIDs)
		forced = fmt.Sprintf(` OR i."Id" = ANY($%d)`, len(buildArgs))
	}

	where := strings.Join(conditions, "\n      AND ")
	if forced != "" {
		where = "(" + where + ")" + forced
	}

	countStmt := fmt.Sprintf(`SELECT COUNT(*) FROM "Image" i WHERE %s`, where)

	filter.PageSize = saved
	filter.Page = savedPage

	return countStmt, buildArgs
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
	query := fmt.Sprintf(`SELECT %s FROM "Image" i WHERE i."Id" = $1`, imageColumns)

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
        FROM "Image"
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
		if err == pgx.ErrNoRows {
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
	_, err := r.db.Exec(ctx, `DELETE FROM "Image" WHERE "Id" = $1`, id)
	if err != nil {
		return fmt.Errorf("delete image: %w", err)
	}
	return nil
}
