package repo

import (
	"context"
	"fmt"

	"waifuapi/internal/store/models"
)

// catalogueKey is the storage key for an image row.
//
// S3Service writes "{id}{extension}" at the bucket root, so this reconstruction
// is what storage is expected to contain.
func catalogueKey(id int64, extension string) string {
	return fmt.Sprintf("%d%s", id, extension)
}

// ListCatalogueKeys returns every image's expected storage key.
//
// The comparison is done in Go rather than SQL because the other half of the
// comparison is a bucket listing, which is not a query. Pulling id and extension
// is cheap next to a listing.
func (r *Repo) ListCatalogueKeys(ctx context.Context) (map[string]int64, error) {
	rows, err := r.db.Query(ctx, `SELECT "Id", "Extension" FROM "Images"`)
	if err != nil {
		return nil, fmt.Errorf("list catalogue keys: %w", err)
	}
	defer rows.Close()

	keys := make(map[string]int64)
	for rows.Next() {
		var id int64
		var extension string
		if err := rows.Scan(&id, &extension); err != nil {
			return nil, fmt.Errorf("scan catalogue key: %w", err)
		}
		keys[catalogueKey(id, extension)] = id
	}
	return keys, rows.Err()
}

// DeleteImages removes catalogue rows in bulk.
//
// Used by the storage reconciliation tool, which can select many rows at once.
// The stored objects are deliberately left alone: the operator chose this side of
// the reconciliation, and removing the object as well would make the action
// irreversible.
//
// Any pending report against a deleted image is resolved rather than left
// dangling. The report row survives its image — the foreign key is nulled on
// delete — so without this the reporter would be left waiting on a moderation
// decision about an image that no longer exists.
func (r *Repo) DeleteImages(ctx context.Context, ids []int64) (int64, error) {
	if len(ids) == 0 {
		return 0, nil
	}

	tx, err := r.db.Begin(ctx)
	if err != nil {
		return 0, fmt.Errorf("begin image delete: %w", err)
	}
	defer func() { _ = tx.Rollback(context.WithoutCancel(ctx)) }()

	const resolvedNote = "The reported image was deleted by a moderator."

	if _, err := tx.Exec(ctx, `
        UPDATE "Reports"
        SET "Status" = $1, "ReviewerNote" = $2
        WHERE "ImageId" = ANY($3) AND "Status" = $4`,
		int(models.ReportResolved), resolvedNote, ids,
		int(models.ReportPending)); err != nil {
		return 0, fmt.Errorf("resolve reports: %w", err)
	}

	tag, err := tx.Exec(ctx, `DELETE FROM "Images" WHERE "Id" = ANY($1)`, ids)
	if err != nil {
		return 0, fmt.Errorf("delete images: %w", err)
	}

	if err := tx.Commit(ctx); err != nil {
		return 0, fmt.Errorf("commit image delete: %w", err)
	}
	return tag.RowsAffected(), nil
}
