// Command backfill recomputes Images.PerceptualHash for every stored image.
//
// # Why
//
// The C# service does not reproduce its own perceptual hashes: ImageSharp's
// resampler accumulates with Vector256.MultiplyAddEstimate when AVX2 is present
// and a scalar float loop when it is not, so the roughly 4,285 stored values
// are a mixture of at least two algorithms. Measured against them, this
// implementation lands 22-38 bits away on average, where the duplicate
// threshold is 4 — so after cutover a re-upload succeeds silently and nobody
// is told.
//
// There is no stable target to port to, because the far side moves with the
// CPU. The only way out is to recompute the whole column with one algorithm.
// This program does that, using the same package the service uses, so the
// values it writes are by construction the ones the running service would
// produce.
//
// # Before running this against production
//
//  1. Apply the schema change. PerceptualHashLegacy must exist, because the
//     C#-produced values are the only record of what the previous
//     implementation computed and this program overwrites them.
//  2. Capture the legacy values first — see the oracle tool, and step 3 of
//     internal/images/phash/MIGRATION.md.
//  3. Run with -dry-run and read the output. The distance histogram is the
//     real check: 22-38 bits confirms this program is hashing the same images
//     the column describes, and distances near zero mean it is re-reading
//     something else entirely.
//
// # Usage
//
//	go run ./tools/backfill -dry-run
//	go run ./tools/backfill -concurrency 8
//	go run ./tools/backfill -after-id 1200        # resume an interrupted run
//
// Configuration is read from the environment exactly as the service reads it,
// so ConnectionStrings__DefaultConnection and the S3__* variables must be set.
package main

import (
	"context"
	"errors"
	"flag"
	"fmt"
	"math/bits"
	"os"
	"os/signal"

	"github.com/jackc/pgx/v5"
	"sort"
	"sync"
	"syscall"
	"time"

	"waifuapi/internal/config"
	"waifuapi/internal/imageproc"
	"waifuapi/internal/storage"
	"waifuapi/internal/store"
	"waifuapi/internal/store/models"
)

// batchSize is how many rows one UPDATE covers.
//
// Deliberately not "one transaction for the whole run" as MIGRATION.md suggests.
// That would hold a database connection and its locks open across roughly 4,285
// object-storage reads, which is minutes of an idle transaction pinning a pool
// slot from a backend that is still serving traffic. Hashing happens outside
// any transaction and only the writes are batched.
const batchSize = 50

// maxReportDistance bounds the distance histogram. Anything beyond this is
// counted in overflow, because a genuine bug produces distances near 64 and a
// full 64-bucket table would be unreadable.
const maxReportDistance = 40

// imageRow is what the program needs about one stored image.
type imageRow struct {
	ID        int64
	Extension string
	Current   uint64
}

// result is the outcome for one row.
type result struct {
	id      int64
	current uint64
	updated uint64
	err     error
}

func main() {
	var (
		dryRun      = flag.Bool("dry-run", false, "read and hash but write nothing")
		concurrency = flag.Int("concurrency", 8, "objects fetched at once")
		afterID     = flag.Int64("after-id", 0, "only rows with a greater Id; used to resume")
		limit       = flag.Int("limit", 0, "stop after this many rows; 0 means all of them")
		sampleSize  = flag.Int("sample", 10, "before/after pairs printed in dry-run mode")
	)
	flag.Parse()

	if err := run(*dryRun, *concurrency, *afterID, *limit, *sampleSize); err != nil {
		fmt.Fprintln(os.Stderr, "backfill:", err)
		os.Exit(1)
	}
}

func run(dryRun bool, concurrency int, afterID int64, limit, sampleSize int) error {
	if concurrency < 1 {
		return errors.New("-concurrency must be at least 1")
	}

	ctx, stop := signal.NotifyContext(context.Background(), os.Interrupt, syscall.SIGTERM)
	defer stop()

	cfg, err := config.Load(config.NewLoader())
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	db, err := store.Open(ctx, store.Config{URL: cfg.DatabaseURL, MaxConnections: 4})
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer db.Close()

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
		return fmt.Errorf("configure object storage: %w", err)
	}

	rows, err := loadRows(ctx, db, afterID, limit)
	if err != nil {
		return fmt.Errorf("read image rows: %w", err)
	}
	if len(rows) == 0 {
		fmt.Println("no rows selected; nothing to do")
		return nil
	}

	mode := "writing"
	if dryRun {
		mode = "dry run, writing nothing"
	}
	fmt.Printf("%d rows selected (after id %d, limit %d), %s\n", len(rows), afterID, limit, mode)

	// This program overwrites the only record of what the C# implementation
	// computed. If that record has not been captured yet, running this destroys
	// it with no way back, so the check happens before any write rather than
	// being left to the runbook.
	if err := checkLegacyCaptured(ctx, db, dryRun); err != nil {
		return err
	}

	started := time.Now()
	results := process(ctx, files, rows, concurrency)

	if dryRun {
		printSample(results, sampleSize)
	}

	summary, err := report(ctx, db, results, rows, dryRun)
	if err != nil {
		return err
	}

	fmt.Printf("\ndone in %s\n", time.Since(started).Round(time.Second))
	printHistogram(summary.histogram, summary.overflow)
	printFailures(summary.failures)

	if summary.failed > 0 {
		// Not an error: a missing object or an undecodable image leaves its row
		// holding a value from the old algorithm, which is the condition this
		// whole exercise exists to remove. It must be noticed, so the exit code
		// is non-zero even though the run itself succeeded.
		return fmt.Errorf("%d of %d rows could not be hashed; re-run with -after-id to resume",
			summary.failed, len(rows))
	}
	return nil
}

// checkLegacyCaptured refuses to write until the C#-produced fingerprints have
// been preserved in PerceptualHashLegacy.
//
// Without this the tool is a one-way door: the column it is about to overwrite
// was never self-reproducing, so the legacy values cannot be recomputed once
// they are gone. A dry run is allowed through either check, since it changes
// nothing and is the natural way to find out whether the schema has been
// applied yet.
func checkLegacyCaptured(ctx context.Context, db *store.DB, dryRun bool) error {
	const query = `
		SELECT count(*), count("PerceptualHashLegacy")
		FROM "Images"`

	var total, captured int64
	if err := db.Pool().QueryRow(ctx, query).Scan(&total, &captured); err != nil {
		if errors.Is(err, pgx.ErrNoRows) {
			return err
		}
		return fmt.Errorf(
			"cannot read Images.PerceptualHashLegacy: %w\n"+
				"apply the schema change first: the C#-produced fingerprints must be\n"+
				"preserved before this column is overwritten, and they cannot be recomputed",
			err)
	}

	if captured == 0 && total > 0 {
		message := fmt.Sprintf(
			"PerceptualHashLegacy is empty for all %d rows; run the oracle backfill "+
				"(internal/images/phash/MIGRATION.md, step 3) before overwriting", total)
		if dryRun {
			fmt.Fprintln(os.Stderr, "warning:", message)
			return nil
		}
		return errors.New(message)
	}

	fmt.Printf("legacy fingerprints preserved for %d of %d rows\n", captured, total)
	return nil
}

// loadRows reads the id, extension and current fingerprint for every image.
func loadRows(ctx context.Context, db *store.DB, afterID int64, limit int) ([]imageRow, error) {
	// The column is cast to text so pgx hands back a bit string rather than raw
	// bytes; models.PerceptualHashFromBitString accepts both, but being explicit
	// keeps the conversion the same on every codec setting.
	const query = `
		SELECT "Id", "Extension", "PerceptualHash"::text
		FROM "Images"
		WHERE "Id" > $1
		ORDER BY "Id"`

	rows, err := db.Pool().Query(ctx, query, afterID)
	if err != nil {
		return nil, err
	}
	defer rows.Close()

	var out []imageRow
	for rows.Next() {
		var (
			row     imageRow
			bitText string
		)
		if err := rows.Scan(&row.ID, &row.Extension, &bitText); err != nil {
			return nil, err
		}

		current, err := models.PerceptualHashFromBitString(bitText)
		if err != nil {
			// A row whose stored value cannot be read is still worth
			// backfilling, and reporting it as unreadable would hide it behind a
			// count that looks like a missing object. The value is reported as
			// zero, which only affects the before/after sample.
			fmt.Fprintf(os.Stderr, "warning: id %d has an unreadable stored hash: %v\n", row.ID, err)
		}
		row.Current = current
		out = append(out, row)

		if limit > 0 && len(out) >= limit {
			break
		}
	}
	return out, rows.Err()
}

// process fetches and hashes every row, at most concurrency at a time.
func process(ctx context.Context, files storage.Store, rows []imageRow, concurrency int) []result {
	results := make([]result, 0, len(rows))
	jobs := make(chan imageRow)

	var (
		wg    sync.WaitGroup
		mu    sync.Mutex
		first = true
	)

	for range concurrency {
		wg.Add(1)
		go func() {
			defer wg.Done()
			for row := range jobs {
				res := hashOne(ctx, files, row)

				mu.Lock()
				// Progress on one line, so a long run is not silent.
				if first {
					first = false
				} else {
					fmt.Print("\r")
				}
				fmt.Printf("  %d/%d  id %d", len(results), len(rows), row.ID)
				mu.Unlock()

				results = append(results, res)
			}
		}()
	}

	for _, row := range rows {
		select {
		case <-ctx.Done():
			close(jobs)
			wg.Wait()
			return results
		case jobs <- row:
		}
	}
	close(jobs)
	wg.Wait()

	fmt.Println()
	return results
}

// hashOne fetches one image and computes its fingerprint.
func hashOne(ctx context.Context, files storage.Store, row imageRow) result {
	res := result{id: row.ID, current: row.Current}

	// GetWithFallback, not Get: PNGs are stored as WebP, so a plain Get misses
	// every PNG row and reports it as absent.
	key := storage.ObjectKey(row.ID, storage.NormalizeExtension(row.Extension))
	data, actualKey, err := files.GetWithFallback(ctx, row.ID, row.Extension)
	if err != nil {
		res.err = fmt.Errorf("fetch %s: %w", key, err)
		return res
	}

	hash, err := imageproc.Hash(data)
	if err != nil {
		res.err = fmt.Errorf("hash %s: %w", actualKey, err)
		return res
	}

	res.updated = hash
	return res
}

// summary aggregates one run.
type summary struct {
	histogram map[int]int
	overflow  int
	failures  []result
	failed    int
}

// report writes the computed fingerprints back, in batches, and gathers the
// distance histogram.
//
// The histogram is the reason this program reports what it does rather than
// just a success count: if the distances come back near zero the tool is not
// hashing the images the column describes, and if they cluster at 64 something
// is badly wrong. Around 22-38 is the expected, already-documented result.
func report(ctx context.Context, db *store.DB, results []result, rows []imageRow, dryRun bool) (summary, error) {
	out := summary{histogram: make(map[int]int)}

	ids := make(map[int64]imageRow, len(rows))
	for _, row := range rows {
		ids[row.ID] = row
	}

	pending := make([]result, 0, batchSize)
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		defer func() { pending = pending[:0] }()

		if !dryRun {
			if err := writeBatch(ctx, db, pending); err != nil {
				return err
			}
		}

		for _, res := range pending {
			if res.err != nil {
				out.failed++
				out.failures = append(out.failures, res)
				continue
			}
			distance := bits.OnesCount64(res.current ^ res.updated)
			switch {
			case distance > maxReportDistance:
				out.overflow++
			default:
				out.histogram[distance]++
			}
		}
		return nil
	}

	for _, res := range results {
		pending = append(pending, res)
		if len(pending) == batchSize {
			if err := flush(); err != nil {
				return out, err
			}
		}
	}
	if err := flush(); err != nil {
		return out, err
	}
	return out, nil
}

// writeBatch updates one batch of rows inside a single transaction.
//
// The values are written from the batch itself rather than from a recomputed
// hash, so a row whose fetch failed keeps whatever it already had instead of
// being zeroed.
func writeBatch(ctx context.Context, db *store.DB, batch []result) error {
	tx, err := db.Pool().Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	const update = `UPDATE "Images" SET "PerceptualHash" = $1::bit(64) WHERE "Id" = $2`

	for _, res := range batch {
		if res.err != nil {
			continue
		}
		bits64 := models.PerceptualHashToBitString(res.updated)
		if _, err := tx.Exec(ctx, update, bits64, res.id); err != nil {
			return fmt.Errorf("update id %d: %w", res.id, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}
	return nil
}

// printSample shows the first few before/after pairs, which is the fastest way
// to confirm the tool is looking at the right images.
func printSample(results []result, count int) {
	shown := 0
	fmt.Println("\nbefore -> after (bit distance)")
	for _, res := range results {
		if res.err != nil || shown >= count {
			continue
		}
		fmt.Printf("  id %-8d %016X -> %016X  (%d)\n",
			res.id, res.current, res.updated, bits.OnesCount64(res.current^res.updated))
		shown++
	}
}

func printHistogram(histogram map[int]int, overflow int) {
	if len(histogram) == 0 && overflow == 0 {
		return
	}

	distances := make([]int, 0, len(histogram))
	for distance := range histogram {
		distances = append(distances, distance)
	}
	sort.Ints(distances)

	total := overflow
	for _, n := range histogram {
		total += n
	}

	fmt.Println("\ndistance between stored and recomputed fingerprint")
	widest := len(fmt.Sprint(distances[len(distances)-1]))
	for _, distance := range distances {
		fmt.Printf("  %*d  %s %d", widest, distance, bar(histogram[distance], histogram[distances[0]]), histogram[distance])
	}
	if overflow > 0 {
		fmt.Printf("  >%d %s %d\n", maxReportDistance, bar(overflow, histogram[distances[0]]), overflow)
	}
	fmt.Printf("\n  %d compared\n", total)
}

// bar renders a proportional bar so the shape of the distribution is readable
// at a glance.
func bar(count, smallest int) string {
	if smallest <= 0 || count <= 0 {
		return ""
	}
	width := 40 * count / smallest
	if width > 40 {
		width = 40
	}
	out := make([]byte, width)
	for i := range out {
		out[i] = '#'
	}
	return string(out) + " "
}

func printFailures(failures []result) {
	if len(failures) == 0 {
		return
	}
	fmt.Printf("\n%d rows could not be hashed:\n", len(failures))
	for _, res := range failures {
		fmt.Printf("  id %-8d %v\n", res.id, res.err)
	}
}
