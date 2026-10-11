package repo

import (
	"context"
	"fmt"
	"time"

	"waifuapi/internal/store/models"
)

// This file loads the relations an image response needs: its tags, its artists,
// and the per-caller fields (favorites, likedAt, albums, addedToAlbumAt).
//
// It exists as its own file because GET /images is the hot path and these are
// the queries that decide whether it costs four round trips per image or four
// round trips per page. They are batched per page rather than per image, which
// is the difference between 30 and 120 statements for a default-sized page.

// ChildVisibility says which review states of tags and artists to include on an
// image.
//
// It mirrors ApplyChildrenReviewStatusIncludes: the default shows accepted
// children only, a moderator can ask for pending or all, and any caller can ask
// for their own pending children alongside the accepted ones.
type ChildVisibility struct {
	Filter models.ReviewStatusFilter

	// ViewerID is the caller, needed only when IncludeMine is true.
	ViewerID int64

	IncludeMine bool
}

// ImageRelations is everything about a page of images that is not on the Image
// row itself.
type ImageRelations struct {
	// TagsByImage and ArtistsByImage are keyed by image id. An image with no
	// visible children is absent from the map; callers must substitute an empty
	// slice rather than emitting null.
	TagsByImage    map[int64][]models.Tag
	ArtistsByImage map[int64][]models.Artist

	// Favorites counts every user who has the image in their default album.
	Favorites map[int64]int64

	// LikedAt is when the viewer added it to their own default album, and is nil
	// when they have not.
	LikedAt map[int64]time.Time

	// AlbumsByImage lists the viewer's own albums containing the image.
	AlbumsByImage map[int64][]Album

	// AddedToAlbumAt is when it entered the album being viewed, and is only
	// populated for the album endpoint.
	AddedToAlbumAt map[int64]time.Time
}

// GetTagsForImages loads the visible tags of a page of images in one statement.
func (r *Repo) GetTagsForImages(ctx context.Context, imageIDs []int64, visibility ChildVisibility) (map[int64][]models.Tag, error) {
	if len(imageIDs) == 0 {
		return map[int64][]models.Tag{}, nil
	}

	where, args := childReviewClause(&visibility, "t")

	args = append(args, imageIDs)
	statement := fmt.Sprintf(`
        SELECT it."ImagesId", t."Id", t."Name", t."Slug", t."Description",
               t."CreatorId", t."ReviewStatus"
        FROM "ImageTag" it
        JOIN "Tags" t ON t."Id" = it."TagsId"
        WHERE it."ImagesId" = ANY($%d) AND %s
        ORDER BY t."Name" ASC`, len(args), where)

	rows, err := r.db.Query(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("get tags for images: %w", err)
	}
	defer rows.Close()

	out := make(map[int64][]models.Tag, len(imageIDs))
	for rows.Next() {
		var imageID int64
		var tag models.Tag
		if err := rows.Scan(&imageID, &tag.ID, &tag.Name, &tag.Slug, &tag.Description,
			&tag.CreatorID, &tag.ReviewStatus); err != nil {
			return nil, fmt.Errorf("scan image tag: %w", err)
		}
		out[imageID] = append(out[imageID], tag)
	}
	return out, rows.Err()
}

// GetArtistsForImages loads the visible artists of a page of images.
func (r *Repo) GetArtistsForImages(ctx context.Context, imageIDs []int64, visibility ChildVisibility) (map[int64][]models.Artist, error) {
	if len(imageIDs) == 0 {
		return map[int64][]models.Artist{}, nil
	}

	where, args := childReviewClause(&visibility, "a")

	args = append(args, imageIDs)
	statement := fmt.Sprintf(`
        SELECT ai."ImagesId", a."Id", a."Name", a."DeviantArt", a."Twitter",
               a."Patreon", a."Pixiv", a."CreatorId", a."ReviewStatus"
        FROM "ArtistImage" ai
        JOIN "Artists" a ON a."Id" = ai."ArtistsId"
        WHERE ai."ImagesId" = ANY($%d) AND %s
        ORDER BY a."Name" ASC`, len(args), where)

	rows, err := r.db.Query(ctx, statement, args...)
	if err != nil {
		return nil, fmt.Errorf("get artists for images: %w", err)
	}
	defer rows.Close()

	out := make(map[int64][]models.Artist, len(imageIDs))
	for rows.Next() {
		var imageID int64
		var artist models.Artist
		if err := rows.Scan(&imageID, &artist.ID, &artist.Name, &artist.DeviantArt,
			&artist.Twitter, &artist.Patreon, &artist.Pixiv,
			&artist.CreatorID, &artist.ReviewStatus); err != nil {
			return nil, fmt.Errorf("scan image artist: %w", err)
		}
		out[imageID] = append(out[imageID], artist)
	}
	return out, rows.Err()
}

// childReviewClause renders the review-status predicate shared by the tag and
// artist child queries.
//
// alias is the table alias so the same expression serves both.
func childReviewClause(visibility *ChildVisibility, alias string) (string, []any) {
	switch visibility.Filter {
	case models.FilterAll:
		return "TRUE", nil

	case models.FilterPending:
		return fmt.Sprintf(`%s."ReviewStatus" = $1`, alias), []any{int(models.ReviewPending)}

	default:
		// Accepted, plus the caller's own pending children when asked. This is
		// why the upload screen can show a freshly created tag on the image it
		// just created without waiting for moderation.
		if visibility.IncludeMine && visibility.ViewerID != 0 {
			return fmt.Sprintf(
					`(%s."ReviewStatus" = $1 OR (%s."ReviewStatus" = $2 AND %s."CreatorId" = $3))`,
					alias, alias, alias),
				[]any{int(models.ReviewAccepted), int(models.ReviewPending), visibility.ViewerID}
		}
		return fmt.Sprintf(`%s."ReviewStatus" = $1`, alias), []any{int(models.ReviewAccepted)}
	}
}

// GetFavorites counts default-album entries per image.
//
// This is a site-wide count, not the viewer's: "favorites" on an image is how
// many people liked it, which is what the frontend renders next to the heart.
func (r *Repo) GetFavorites(ctx context.Context, imageIDs []int64) (map[int64]int64, error) {
	if len(imageIDs) == 0 {
		return map[int64]int64{}, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT ai."ImageId", COUNT(*)
        FROM "AlbumItems" ai
        JOIN "Albums" a ON a."Id" = ai."AlbumId"
        WHERE ai."ImageId" = ANY($1) AND a."IsDefault" = true
        GROUP BY ai."ImageId"`, imageIDs)
	if err != nil {
		return nil, fmt.Errorf("get favorites: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]int64, len(imageIDs))
	for rows.Next() {
		var id, count int64
		if err := rows.Scan(&id, &count); err != nil {
			return nil, fmt.Errorf("scan favorites: %w", err)
		}
		out[id] = count
	}
	return out, rows.Err()
}

// GetViewerLikes reports when the viewer added each image to their own default
// album.
func (r *Repo) GetViewerLikes(ctx context.Context, imageIDs []int64, viewerID int64) (map[int64]time.Time, error) {
	if viewerID == 0 || len(imageIDs) == 0 {
		return map[int64]time.Time{}, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT ai."ImageId", ai."AddedAt"
        FROM "AlbumItems" ai
        JOIN "Albums" a ON a."Id" = ai."AlbumId"
        WHERE ai."ImageId" = ANY($1) AND a."UserId" = $2 AND a."IsDefault" = true`,
		imageIDs, viewerID)
	if err != nil {
		return nil, fmt.Errorf("get viewer likes: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]time.Time, len(imageIDs))
	for rows.Next() {
		var id int64
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, fmt.Errorf("scan viewer like: %w", err)
		}
		out[id] = at
	}
	return out, rows.Err()
}

// GetViewerAlbums lists the viewer's own albums containing each image.
//
// Only the viewer's albums are returned: the albums field drives the "in album"
// markers on the image page, and showing a stranger's album would leak the
// existence of private collections.
func (r *Repo) GetViewerAlbums(ctx context.Context, imageIDs []int64, viewerID int64) (map[int64][]Album, error) {
	if viewerID == 0 || len(imageIDs) == 0 {
		return map[int64][]Album{}, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT ai."ImageId", a."Id", a."UserId", a."Name", a."Description",
               a."IsDefault", COUNT(ai2."ImageId")
        FROM "AlbumItems" ai
        JOIN "Albums" a ON a."Id" = ai."AlbumId"
        LEFT JOIN "AlbumItems" ai2 ON ai2."AlbumId" = a."Id"
        WHERE ai."ImageId" = ANY($1) AND a."UserId" = $2
        GROUP BY ai."ImageId", a."Id", a."UserId", a."Name", a."Description", a."IsDefault"
        ORDER BY a."IsDefault" DESC, a."Name" ASC`,
		imageIDs, viewerID)
	if err != nil {
		return nil, fmt.Errorf("get viewer albums: %w", err)
	}
	defer rows.Close()

	out := make(map[int64][]Album, len(imageIDs))
	for rows.Next() {
		var imageID int64
		var album Album
		var count int64
		if err := rows.Scan(&imageID, &album.ID, &album.UserID, &album.Name,
			&album.Description, &album.IsDefault, &count); err != nil {
			return nil, fmt.Errorf("scan viewer album: %w", err)
		}
		album.ImageCount = int(count)
		out[imageID] = append(out[imageID], album)
	}
	return out, rows.Err()
}

// GetAddedToAlbumAt reports when each image entered the album being viewed.
//
// Only the album endpoint populates this; on /images it stays empty and the
// field serialises as null.
func (r *Repo) GetAddedToAlbumAt(ctx context.Context, imageIDs []int64, albumID int64) (map[int64]time.Time, error) {
	if albumID == 0 || len(imageIDs) == 0 {
		return map[int64]time.Time{}, nil
	}

	rows, err := r.db.Query(ctx, `
        SELECT "ImageId", "AddedAt" FROM "AlbumItems"
        WHERE "ImageId" = ANY($1) AND "AlbumId" = $2`, imageIDs, albumID)
	if err != nil {
		return nil, fmt.Errorf("get added to album: %w", err)
	}
	defer rows.Close()

	out := make(map[int64]time.Time, len(imageIDs))
	for rows.Next() {
		var id int64
		var at time.Time
		if err := rows.Scan(&id, &at); err != nil {
			return nil, fmt.Errorf("scan added to album: %w", err)
		}
		out[id] = at
	}
	return out, rows.Err()
}
