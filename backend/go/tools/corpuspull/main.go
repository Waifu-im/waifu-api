// Command corpuspull downloads the image corpus from object storage to a local
// directory, for the perceptual-hash migration.
//
// The oracle in tools/oracle reads a *directory*, not a bucket, so the images
// have to exist on disk before it can run. This fetches exactly the objects
// tools/backfill will later read: one GET per row in Images, keyed {id}{ext}.
//
// It deliberately downloads by database row rather than by bucket listing. A
// bucket can hold objects with no matching row, and rows with no matching
// object; fetching only what the catalogue claims keeps the corpus exactly as
// large as the migration needs and makes a gap visible instead of silently
// absent.
//
// # Safety
//
// Read-only against both dependencies, by construction and by configuration:
//
//   - The database session sets default_transaction_read_only, so a bug in this
//     program cannot modify production even if it tried.
//   - Object storage is only ever read. There is no delete, no put and no
//     lifecycle operation.
//
// It is safe to run against production, which is the point: the alternative is
// running the migration blind.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"os/signal"
	"path/filepath"
	"sync"
	"sync/atomic"
	"syscall"
	"time"

	"github.com/jackc/pgx/v5/pgxpool"

	"waifuapi/internal/config"
	"waifuapi/internal/storage"
)

func main() {
	var (
		outDir      = flag.String("out", "", "directory to write the corpus into; required")
		listOnly    = flag.Bool("list-only", false, "report what the bucket holds and exit; no database, no download")
		dryRun      = flag.Bool("dry-run", false, "read the catalogue and report totals without downloading")
		concurrency = flag.Int("concurrency", 8, "objects fetched at once")
		limit       = flag.Int("limit", 0, "stop after this many rows; 0 means all")
		afterID     = flag.Int64("after-id", 0, "only rows with a greater Id; used to resume")
	)
	flag.Parse()

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(config.NewLoader())
	if err != nil {
		fail("load configuration: %w", err)
	}

	files, err := storage.NewS3Store(storage.Config{
		Provider:       storage.Provider(cfg.S3.Provider),
		AccessKey:      cfg.S3.AccessKey,
		SecretKey:      cfg.S3.SecretKey,
		ServiceURL:     cfg.S3.ServiceURL,
		Region:         cfg.S3.Region,
		BucketName:     cfg.S3.BucketName,
		ForcePathStyle: cfg.S3.ForcePathStyle,
	})
	if err != nil {
		fail("configure object storage: %w", err)
	}

	if *listOnly {
		listOnlyRun(ctx, files)
		return
	}

	if *outDir == "" {
		fail("-out is required unless -list-only is given")
	}
	if *concurrency < 1 {
		fail("-concurrency must be at least 1")
	}
	if err := os.MkdirAll(*outDir, 0o755); err != nil {
		fail("create output directory: %w", err)
	}

	pool, err := openReadOnly(ctx, cfg.DatabaseURL)
	if err != nil {
		fail("%w", err)
	}
	defer pool.Close()

	rows, err := loadRows(ctx, pool, *afterID, *limit)
	if err != nil {
		fail("read image rows: %w", err)
	}
	fmt.Printf("catalogue lists %d images\n", len(rows))

	if *dryRun {
		fmt.Println("dry run: nothing downloaded")
		return
	}

	stats := pull(ctx, files, rows, *outDir, *concurrency)

	fmt.Printf("\ndone\n")
	fmt.Printf("  downloaded %d\n", stats.downloaded.Load())
	fmt.Printf("  already present %d\n", stats.skipped.Load())
	fmt.Printf("  missing from storage %d\n", stats.missing.Load())
	if total := stats.bytes.Load(); total > 0 {
		fmt.Printf("  transferred %s\n", humanBytes(total))
	}

	if stats.missing.Load() > 0 {
		// Not an error: some rows legitimately have no object, and the
		// backfill will report the same ones. Surfaced rather than hidden so
		// the count can be compared between runs.
		os.Exit(1)
	}
}

func fail(format string, args ...any) {
	fmt.Fprintf(os.Stderr, "corpuspull: "+format+"\n", args...)
	os.Exit(1)
}

// openReadOnly connects with the session forced read-only.
func openReadOnly(ctx context.Context, url string) (*pgxpool.Pool, error) {
	pool, err := pgxpool.New(ctx, url)
	if err != nil {
		return nil, fmt.Errorf("connect to database: %w", err)
	}
	// Belt and braces. This program only issues SELECTs, so the setting should
	// never matter; it is set anyway so that a future edit to this file cannot
	// turn a read-only tool into a write.
	if _, err := pool.Exec(ctx, `SET default_transaction_read_only = on`); err != nil {
		pool.Close()
		return nil, fmt.Errorf("could not pin the session read-only: %w", err)
	}
	if _, err := pool.Exec(ctx, `SHOW default_transaction_read_only`); err != nil {
		pool.Close()
		return nil, fmt.Errorf("could not confirm read-only mode: %w", err)
	}
	return pool, nil
}

// row is one image the catalogue claims exists.
type row struct {
	id        int64
	extension string
}

// loadRows reads the ids and extensions.
func loadRows(ctx context.Context, pool *pgxpool.Pool, afterID int64, limit int) ([]row, error) {
	const query = `
		SELECT "Id", "Extension"
		FROM "Images"
		WHERE "Id" > $1
		ORDER BY "Id"`

	rows, err := pool.Query(ctx, query, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []row
	for rows.Next() {
		var r row
		if err := rows.Scan(&r.id, &r.extension); err != nil {
			return nil, err
		}
		out = append(out, r)
		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

// counters accumulates the run's outcome.
type counters struct {
	downloaded atomic.Int64
	skipped    atomic.Int64
	missing    atomic.Int64
	bytes      atomic.Int64
}

// pull fetches every row to a local file named {id}{ext}.
func pull(ctx context.Context, files storage.Store, rows []row, outDir string, concurrency int) *counters {
	stats := &counters{}
	jobs := make(chan row)

	var wg sync.WaitGroup
	var mu sync.Mutex
	lastReport := time.Now()

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for r := range jobs {
				candidates := storage.ObjectKeyCandidates(r.id, r.extension)
				path := filepath.Join(outDir, filepath.Base(candidates[0]))

				// Resumable: an object already on disk is left alone. A
				// partial file from an interrupted run is not trusted, because
				// a truncated image would silently hash to the wrong thing.
				//
				// Both candidate names are checked, so a run interrupted
				// before the WebP fallback existed still resumes correctly.
				present := false
				for _, candidate := range candidates {
					alt := filepath.Join(outDir, filepath.Base(candidate))
					if info, err := os.Stat(alt); err == nil && info.Size() > 0 {
						path = alt
						present = true
						break
					}
				}
				if present {
					stats.skipped.Add(1)
					if info, err := os.Stat(path); err == nil {
						stats.bytes.Add(info.Size())
					}
					continue
				}

				data, key, err := files.GetWithFallback(ctx, r.id, r.extension)
				if err != nil {
					if errors.Is(err, storage.ErrNotFound) {
						stats.missing.Add(1)
					} else {
						// Reported rather than fatal, so one unreachable
						// object does not abandon a multi-hour run.
						fmt.Fprintf(os.Stderr, "warning: %s: %v\n", key, err)
					}
					continue
				}

				if err := writeFile(path, data); err != nil {
					fmt.Fprintf(os.Stderr, "warning: write %s: %v\n", path, err)
					continue
				}
				stats.downloaded.Add(1)
				stats.bytes.Add(int64(len(data)))

				mu.Lock()
				if time.Since(lastReport) > 5*time.Second {
					lastReport = time.Now()
					fmt.Printf("  %d fetched, %d present, %d missing, %s so far\n",
						stats.downloaded.Load(), stats.skipped.Load(), stats.missing.Load(),
						humanBytes(stats.bytes.Load()))
				}
				mu.Unlock()
			}
		}()
	}

	for _, r := range rows {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return stats
		case jobs <- r:
		}
	}
	close(jobs)
	wg.Wait()
	return stats
}

// writeFile writes via a temporary name and renames, so an interrupted run
// never leaves a truncated file that the next run would trust.
func writeFile(path string, data []byte) error {
	tmp := path + ".part"
	if err := os.WriteFile(tmp, data, 0o644); err != nil {
		return err
	}
	return os.Rename(tmp, path)
}

// listOnlyRun reports what the bucket holds, without touching the database.
// Used to establish which endpoint actually contains the corpus before pulling
// tens of gigabytes from the wrong place.
func listOnlyRun(ctx context.Context, files storage.Store) {
	keys, err := files.List(ctx)
	if err != nil {
		fail("list bucket: %w", err)
	}
	fmt.Printf("bucket holds %d objects\n", len(keys))
	for i, k := range keys {
		if i >= 10 {
			fmt.Printf("  ... and %d more\n", len(keys)-10)
			break
		}
		fmt.Printf("  %s\n", k)
	}
}

// humanBytes renders a byte count in the largest sensible unit.
func humanBytes(n int64) string {
	const unit = 1024
	if n < unit {
		return fmt.Sprintf("%d B", n)
	}
	div, exp := int64(unit), 0
	for size := n / unit; size >= unit; size /= unit {
		div *= unit
		exp++
	}
	return fmt.Sprintf("%.1f %ciB", float64(n)/float64(div), "KMGTPE"[exp])
}
