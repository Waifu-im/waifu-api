// Command legacyload writes the C#-produced perceptual hashes into the frozen
// legacy column.
//
// This is step 3 of internal/images/phash/MIGRATION.md, and it must run before
// tools/backfill, which refuses to write until this column has been populated.
// The ordering is not a formality: the values being preserved here are the only
// record of what the C# implementation computed, they cannot be reproduced —
// ImageSharp's resampler branches on AVX2 — and tools/backfill overwrites the
// column they currently occupy.
//
// # Pipeline
//
//	tools/oracle (C#, run locally in a container)  ->  hashes.tsv
//	legacyload (this)                               ->  PerceptualHashLegacy
//	tools/backfill                                  ->  PerceptualHash
//
// # The oracle's output format
//
// Written by tools/oracle/Program.cs, one line per image, tab-separated:
//
//	relative<TAB>hash<TAB>WxH<TAB>dominant<TAB>isAnimated
//	relative<TAB>DECODE_ERROR<TAB>ExceptionTypeName
//
// The hash is C#'s BitArray.ToString("X16"), so 16 uppercase hex characters.
//
// # Naming requirement
//
// The first column is the oracle's *input* filename, not a database id, so the
// corpus must be named `{id}{ext}` — the same convention as the object storage
// key. A filename whose stem is not an integer is rejected rather than guessed
// at: silently writing a hash to the wrong row is far worse than refusing.
//
// Usage:
//
//	go run ./tools/legacyload -tsv hashes.tsv -dry-run
//	go run ./tools/legacyload -tsv hashes.tsv
package main

import (
	"bufio"
	"context"
	"errors"
	"flag"
	"fmt"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"github.com/jackc/pgx/v5/pgxpool"

	"waifuapi/internal/config"
	"waifuapi/internal/store/models"
)

// batchSize is how many rows one UPDATE covers.
const batchSize = 500

func main() {
	var (
		tsvPath = flag.String("tsv", "", "oracle output file; required")
		dryRun  = flag.Bool("dry-run", false, "read the file but write nothing")
	)
	flag.Parse()

	if *tsvPath == "" {
		fmt.Fprintln(os.Stderr, "legacyload: -tsv is required")
		flag.Usage()
		os.Exit(2)
	}
	if err := run(*tsvPath, *dryRun); err != nil {
		fmt.Fprintln(os.Stderr, "legacyload:", err)
		os.Exit(1)
	}
}

func run(tsvPath string, dryRun bool) error {
	entries, err := parseTSV(tsvPath)
	if err != nil {
		return err
	}
	if len(entries) == 0 {
		return errors.New("no usable rows in the oracle output")
	}

	cfg, err := config.Load(config.NewLoader())
	if err != nil {
		return fmt.Errorf("load configuration: %w", err)
	}

	pool, err := pgxpool.New(context.Background(), cfg.DatabaseURL)
	if err != nil {
		return fmt.Errorf("connect to database: %w", err)
	}
	defer pool.Close()

	skipped, unknown, err := checkAgainstDatabase(context.Background(), pool, entries)
	if err != nil {
		return err
	}
	if len(unknown) > 0 {
		return fmt.Errorf("%d of %d oracle rows name an image id that is not in the "+
			"database, starting at id %d; the corpus naming is probably wrong",
			len(unknown), len(entries), unknown[0])
	}
	if len(skipped) > 0 {
		fmt.Fprintf(os.Stderr,
			"warning: %d rows are already populated and will be left alone\n", len(skipped))
	}

	// Mark the entries so the write loop actually skips them. Without this the
	// loop relies on the WHERE clause to make re-runs harmless while still
	// counting them as written, which reports a number that is not true.
	populated := make(map[int64]bool, len(skipped))
	for _, id := range skipped {
		populated[id] = true
	}
	for i := range entries {
		if populated[entries[i].id] {
			entries[i].alreadySet = true
		}
	}

	mode := "writing"
	if dryRun {
		mode = "dry run, writing nothing"
	}
	fmt.Printf("%d oracle rows, %d to write, %s\n", len(entries), len(entries)-len(skipped), mode)

	pending := make([]entry, 0, batchSize)
	written := 0
	flush := func() error {
		if len(pending) == 0 {
			return nil
		}
		defer func() { pending = pending[:0] }()
		if dryRun {
			return nil
		}
		if err := writeBatch(context.Background(), pool, pending); err != nil {
			return err
		}
		written += len(pending)
		return nil
	}

	for _, e := range entries {
		if e.alreadySet {
			continue
		}
		pending = append(pending, e)
		if len(pending) == batchSize {
			if err := flush(); err != nil {
				return err
			}
		}
	}
	if err := flush(); err != nil {
		return err
	}

	if dryRun {
		fmt.Printf("dry run: %d rows would be written, nothing was\n", len(entries)-len(skipped))
		return nil
	}
	fmt.Printf("wrote %d legacy fingerprints\n", written)
	return nil
}

// entry is one oracle row, resolved to a database id.
type entry struct {
	id         int64
	hash       uint64
	alreadySet bool
}

// parseTSV reads the oracle output.
//
// A row the oracle could not decode is reported rather than failing the run:
// those images will simply keep whatever the legacy column already holds, and
// the Go backfill will refuse to touch rows it cannot read from storage, so they
// stay visibly wrong rather than being silently zeroed.
func parseTSV(path string) ([]entry, error) {
	file, err := os.Open(path)
	if err != nil {
		return nil, err
	}
	defer file.Close()

	var (
		entries      []entry
		decodeFailed int
		badLines     int
		scanner      = bufio.NewScanner(file)
		lineNo       = 0
	)
	scanner.Buffer(make([]byte, 0, 64*1024), 1024*1024)

	for scanner.Scan() {
		lineNo++
		line := strings.TrimRight(scanner.Text(), "\r")
		if strings.TrimSpace(line) == "" {
			continue
		}
		fields := strings.Split(line, "\t")
		if len(fields) < 2 {
			badLines++
			continue
		}

		relative := filepath.ToSlash(fields[0])
		if fields[1] == "DECODE_ERROR" {
			decodeFailed++
			continue
		}

		id, err := idFromFilename(relative)
		if err != nil {
			return nil, fmt.Errorf("line %d: %w", lineNo, err)
		}
		hash, err := models.PerceptualHashFromHex(fields[1])
		if err != nil {
			return nil, fmt.Errorf("line %d (%s): %w", lineNo, relative, err)
		}
		entries = append(entries, entry{id: id, hash: hash})
	}
	if err := scanner.Err(); err != nil {
		return nil, fmt.Errorf("read %s: %w", path, err)
	}

	if decodeFailed > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d rows the oracle could not decode were skipped\n",
			decodeFailed)
	}
	if badLines > 0 {
		fmt.Fprintf(os.Stderr, "warning: %d lines had fewer than two columns and were skipped\n",
			badLines)
	}
	return entries, scanner.Err()
}

// idFromFilename extracts the image id from a corpus filename.
//
// The corpus must be named the way object storage keys are, which is what makes
// the oracle's output mappable to rows at all.
func idFromFilename(relative string) (int64, error) {
	stem := strings.TrimSuffix(relative, filepath.Ext(relative))
	id, err := strconv.ParseInt(stem, 10, 64)
	if err != nil {
		return 0, fmt.Errorf(
			"cannot derive an image id from %q: the oracle reports the input filename, "+
				"so the corpus must be named {id}{ext} like an object storage key", relative)
	}
	return id, nil
}

// checkAgainstDatabase confirms every oracle row maps to a real image, and notes
// which are already populated.
//
// Requiring all of them is deliberate. A partial match almost always means the
// corpus was named wrongly, and writing the rows that happen to line up would
// produce a legacy column that looks populated but is wrong for most rows —
// which is worse than an obvious failure.
func checkAgainstDatabase(ctx context.Context, pool *pgxpool.Pool, entries []entry) (alreadySet, unknown []int64, err error) {
	ids := make([]int64, 0, len(entries))
	for _, e := range entries {
		ids = append(ids, e.id)
	}

	rows, err := pool.Query(ctx,
		`SELECT "Id", ("PerceptualHashLegacy" IS NOT NULL) FROM "Images" WHERE "Id" = ANY($1)`, ids)
	if err != nil {
		return nil, nil, fmt.Errorf("query images: %w", err)
	}
	defer rows.Close()

	seen := make(map[int64]bool, len(entries))
	for rows.Next() {
		var (
			id    int64
			isSet bool
		)
		if err := rows.Scan(&id, &isSet); err != nil {
			continue
		}
		seen[id] = true
		if isSet {
			alreadySet = append(alreadySet, id)
		}
	}
	for _, e := range entries {
		if !seen[e.id] {
			unknown = append(unknown, e.id)
		}
	}
	if err := rows.Err(); err != nil {
		return nil, nil, fmt.Errorf("read images: %w", err)
	}
	return alreadySet, unknown, nil
}

// writeBatch updates one batch inside a single transaction.
func writeBatch(ctx context.Context, pool *pgxpool.Pool, batch []entry) error {
	tx, err := pool.Begin(ctx)
	if err != nil {
		return fmt.Errorf("begin transaction: %w", err)
	}
	defer func() { _ = tx.Rollback(ctx) }()

	// The legacy column is only ever written by this tool, and only where it is
	// NULL, so re-running cannot overwrite a preserved value by accident.
	const update = `
		UPDATE "Images" SET "PerceptualHashLegacy" = $1::bit(64)
		WHERE "Id" = $2 AND "PerceptualHashLegacy" IS NULL`

	for _, e := range batch {
		bits := models.PerceptualHashToBitString(e.hash)
		if _, err := tx.Exec(ctx, update, bits, e.id); err != nil {
			return fmt.Errorf("update id %d: %w", e.id, err)
		}
	}

	if err := tx.Commit(ctx); err != nil {
		return fmt.Errorf("commit batch: %w", err)
	}
	return nil
}
