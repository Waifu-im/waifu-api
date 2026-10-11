package store

import (
	"os"
	"path/filepath"
	"regexp"
	"strings"
	"testing"
)

// efSnapshotPath is the EF Core model snapshot, which is the authoritative
// description of the production database. It sits outside this module because
// it belongs to the C# service being replaced.
//
// backend/go/internal/store -> backend/
const efSnapshotPath = "../../../WaifuApi.Infrastructure/Persistence/Migrations/WaifuDbContextModelSnapshot.cs"

// TestSchemaMatchesEntityFrameworkModel is the guard for the single most
// dangerous class of bug in this rewrite.
//
// A table name that is wrong in the Go SQL is not a compile error and not a test
// failure as long as the test database was created from the same wrong schema.
// It is a runtime "relation does not exist" on every request, against the real
// database, discovered after deployment.
//
// That is not hypothetical. schema.sql was originally derived from the EF
// model's ENTITY names rather than its ToTable names, which made every table
// singular ("Image", "Tag", "User") where production is plural ("Images", "Tags",
// "Users"), and left the ArtistImage/ImageTag join columns singular where
// production names them ArtistsId/ImagesId/TagsId. Fourteen of fourteen tables
// disagreed, and the entire suite was green because the suite created its own
// database from the same mistake.
//
// So this test does not check a hand-written list of names, which would just
// re-encode today's guess. It parses the same EF snapshot the C# service is
// generated from and diffs it against the frozen schema, which means a change on
// the C# side shows up here as a failure rather than as an outage.
//
// It skips when the snapshot is absent, so a Go-only checkout still builds.
func TestSchemaMatchesEntityFrameworkModel(t *testing.T) {
	raw, err := os.ReadFile(filepath.Clean(efSnapshotPath))
	if err != nil {
		t.Skipf("EF model snapshot not available at %s (%v); "+
			"run from the full repository to check schema parity", efSnapshotPath, err)
	}

	want := parseEFModel(string(raw))
	if len(want) == 0 {
		t.Fatal("parsed zero tables from the EF snapshot; the parser or the file format changed")
	}

	got := parseSchemaSQL(readFile(t, "schema.sql"))

	var problems []string
	for table, columns := range want {
		gotColumns, ok := got[table]
		if !ok {
			problems = append(problems, "table "+table+" is in production but missing from schema.sql")
			continue
		}
		for _, column := range sortedKeys(columns) {
			if !gotColumns[column] {
				problems = append(problems,
					"table "+table+" is missing column "+column)
			}
		}
		for _, column := range sortedKeys(gotColumns) {
			if !columns[column] {
				problems = append(problems,
					"table "+table+" has column "+column+", which production does not")
			}
		}
	}
	for table := range got {
		if _, ok := want[table]; !ok {
			problems = append(problems, "table "+table+" is in schema.sql but not in production")
		}
	}

	for _, p := range problems {
		t.Error(p)
	}
	if len(problems) == 0 {
		t.Logf("schema.sql agrees with production on %d tables", len(want))
	}
}

// readFile reads a file beside this source file.
func readFile(t *testing.T, name string) string {
	t.Helper()

	raw, err := os.ReadFile(name)
	if err != nil {
		t.Fatalf("read %s: %v", name, err)
	}
	return string(raw)
}

// parseEFModel extracts table name -> column set from a model snapshot.
//
// Only a property that declares a column type is a real column. Join-table
// foreign keys also appear as plain b.Property lines inside relationship
// blocks, and counting those would invent columns that were never created.
func parseEFModel(snapshot string) map[string]map[string]bool {
	tables := map[string]map[string]bool{}

	blocks := strings.Split(snapshot, `modelBuilder.Entity("`)[1:]
	for _, block := range blocks {
		_, body, found := strings.Cut(block, `", b =>`)
		if !found {
			continue
		}

		table := regexp.MustCompile(`b\.ToTable\("([^"]+)"\)`).FindStringSubmatch(body)
		if table == nil {
			continue
		}

		if tables[table[1]] == nil {
			tables[table[1]] = map[string]bool{}
		}

		property := regexp.MustCompile(`(?s)b\.Property<[^>]+>\("([^"]+)"\)(.*?);`)
		for _, match := range property.FindAllStringSubmatch(body, -1) {
			if strings.Contains(match[2], "HasColumnType") {
				tables[table[1]][match[1]] = true
			}
		}
	}

	return tables
}

// parseSchemaSQL extracts table name -> column set from schema.sql.
func parseSchemaSQL(sql string) map[string]map[string]bool {
	tables := map[string]map[string]bool{}

	create := regexp.MustCompile(`(?s)CREATE TABLE IF NOT EXISTS "([^"]+)"\s*\((.*?)\n\);`)
	column := regexp.MustCompile(`^\s*"([^"]+)"\s+\S`)

	for _, match := range create.FindAllStringSubmatch(sql, -1) {
		tables[match[1]] = map[string]bool{}
		for _, line := range strings.Split(match[2], "\n") {
			if name := column.FindStringSubmatch(line); name != nil {
				tables[match[1]][name[1]] = true
			}
		}
	}

	return tables
}

// sortedKeys gives map iteration a deterministic order, so a schema mismatch
// always reports the same first failure instead of varying run to run.
func sortedKeys(m map[string]bool) []string {
	keys := make([]string, 0, len(m))
	for k := range m {
		keys = append(keys, k)
	}
	for i := 1; i < len(keys); i++ {
		for j := i; j > 0 && keys[j] < keys[j-1]; j-- {
			keys[j], keys[j-1] = keys[j-1], keys[j]
		}
	}
	return keys
}
