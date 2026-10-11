// Package storage uploads image bytes to object storage.
//
// # Why an interface
//
// The upload handler needs to write a file and then be able to fail the whole
// operation. Both the C# service and this one handle that with a transaction
// plus a compensating delete. Keeping storage behind an interface means the
// handler can be tested without a running MinIO or a cloud bucket, which
// matters because the interesting behaviour is the rollback, not the PUT.
package storage

import (
	"context"
	"errors"
	"fmt"
	"io"
	"strings"
)

// ErrNotFound is returned when an object does not exist.
var ErrNotFound = errors.New("object not found")

// Store is the subset of object storage this service needs.
type Store interface {
	// Put writes an object. It must be safe to call with a reader that yields
	// an error partway through, in which case nothing should be committed.
	Put(ctx context.Context, key string, body io.Reader, contentType string) error

	// Delete removes an object.
	//
	// Deleting something that is already absent is not an error, because the
	// rollback path calls this after a partial failure and must be idempotent.
	Delete(ctx context.Context, key string) error

	// Exists reports whether an object is present.
	Exists(ctx context.Context, key string) (bool, error)
}

// ObjectKey builds the storage key for an image.
//
// Exported because the upload handler builds the same key in order to delete it
// when a partial failure needs unwinding.
//
// The C# service stores images as "{id}.{extension}" at the bucket root, and
// the Cloudflare Worker in front of storage reconstructs that path, so the key
// format is part of the contract and must not change.
func ObjectKey(id int64, extension string) string {
	return fmt.Sprintf("%d%s", id, extension)
}

// NormalizeExtension returns a lower-cased extension including the leading dot,
// defaulting to ".jpg" when the supplied value is unusable.
//
// A missing extension would produce a key the CDN Worker cannot serve, so this
// falls back rather than writing an unreachable object.
func NormalizeExtension(extension string) string {
	cleaned := strings.ToLower(strings.TrimSpace(extension))
	if cleaned == "" {
		return ".jpg"
	}
	if !strings.HasPrefix(cleaned, ".") {
		cleaned = "." + cleaned
	}
	// Guard against a value that is only dots, or contains a path separator
	// which would let a crafted extension escape the intended key.
	if strings.Trim(cleaned, ".") == "" || strings.ContainsAny(cleaned, "/\\") {
		return ".jpg"
	}
	return cleaned
}
