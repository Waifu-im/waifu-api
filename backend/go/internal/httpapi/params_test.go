package httpapi

import (
	"bytes"
	"encoding/json"
	"net/http/httptest"
	"strings"
	"testing"

	"waifuapi/internal/config"
)

// jsonMarshal and contains keep the empty-array assertion readable.
func jsonMarshal(value any) ([]byte, error) { return json.Marshal(value) }

func contains(haystack, needle string) bool { return strings.Contains(haystack, needle) }

var _ = bytes.MinRead

// testLimits mirrors the production image settings: one row per request, and no
// ceiling because MaxPageSize is -1.
var testLimits = config.PageSize{Default: 1, Max: -1}

func TestParsePaginationDefaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/images", nil)

	page, err := parsePagination(r, testLimits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Page != 0 {
		t.Errorf("default page = %d, want 0 (first page is index 0 internally)", page.Page)
	}
	if page.PageSize != testLimits.Default {
		t.Errorf("default page size = %d, want %d", page.PageSize, testLimits.Default)
	}
	if page.offset() != 0 {
		t.Errorf("offset of first page = %d, want 0", page.offset())
	}
	if page.pageNumber() != 1 {
		t.Errorf("pageNumber = %d, want 1", page.pageNumber())
	}
}

func TestParsePaginationConvertsOneBasedPage(t *testing.T) {
	// The API is one-based because clients expect it, but SQL OFFSET is
	// zero-based. Getting this wrong silently returns the wrong page.
	r := httptest.NewRequest("GET", "/images?page=3&pageSize=20", nil)

	page, err := parsePagination(r, testLimits)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Page != 2 {
		t.Errorf("page 3 should map to index 2, got %d", page.Page)
	}
	if page.offset() != 40 {
		t.Errorf("offset = %d, want 40", page.offset())
	}
}

func TestParsePaginationNormalisesLikePaginationUtils(t *testing.T) {
	// PaginationUtils.Normalize in the C# service is permissive: a
	// non-positive pageSize falls back to the default, and a page below 1
	// becomes page 1. Both behaviours must match, because a client sending
	// pageSize=0 gets the default page in the C# service and would otherwise
	// get an error here.
	cases := []struct {
		target       string
		wantPage     int
		wantPageSize int
	}{
		{"/images?pageSize=0", 0, testLimits.Default},
		{"/images?pageSize=-1", 0, testLimits.Default},
		{"/images?page=0", 0, testLimits.Default},
		{"/images?page=-5", 0, testLimits.Default},
		{"/images?page=2&pageSize=50", 1, 50},
	}

	for _, c := range cases {
		r := httptest.NewRequest("GET", c.target, nil)
		page, err := parsePagination(r, testLimits)
		if err != nil {
			t.Errorf("%s: unexpected error: %v", c.target, err)
			continue
		}
		if page.Page != c.wantPage || page.PageSize != c.wantPageSize {
			t.Errorf("%s: got page=%d size=%d, want page=%d size=%d",
				c.target, page.Page, page.PageSize, c.wantPage, c.wantPageSize)
		}
	}
}

func TestParsePaginationClampsToConfiguredMaximum(t *testing.T) {
	// A positive MaxPageSize caps the page; a non-positive one means "no cap",
	// which is what production sets.
	capped := config.PageSize{Default: 30, Max: 200}

	r := httptest.NewRequest("GET", "/tags?pageSize=100000", nil)
	page, err := parsePagination(r, capped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.PageSize != capped.Max {
		t.Errorf("oversized pageSize should clamp to %d, got %d", capped.Max, page.PageSize)
	}

	uncapped := config.PageSize{Default: 30, Max: -1}
	r = httptest.NewRequest("GET", "/tags?pageSize=100000", nil)
	page, err = parsePagination(r, uncapped)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.PageSize != 100000 {
		t.Errorf("MaxPageSize=-1 means no ceiling, got %d", page.PageSize)
	}
}

func TestParsePaginationRejectsNonNumericInput(t *testing.T) {
	// A value that is not a number at all is still an error: silently ignoring
	// it would return a different result set than the client asked for.
	for _, target := range []string{
		"/images?page=abc",
		"/images?pageSize=abc",
	} {
		r := httptest.NewRequest("GET", target, nil)
		if _, err := parsePagination(r, testLimits); err == nil {
			t.Errorf("%s should have been rejected", target)
		}
	}
}

func TestBuildPaginatedList(t *testing.T) {
	cases := []struct {
		name               string
		page               pageParams
		total              int
		wantPages          int
		wantNext, wantPrev bool
	}{
		{"single page", pageParams{Page: 0, PageSize: 30}, 10, 1, false, false},
		{"first of many", pageParams{Page: 0, PageSize: 30}, 100, 4, true, false},
		{"middle", pageParams{Page: 1, PageSize: 30}, 100, 4, true, true},
		{"last", pageParams{Page: 3, PageSize: 30}, 100, 4, false, true},
		{"exact multiple", pageParams{Page: 0, PageSize: 25}, 100, 4, true, false},
		{"empty result", pageParams{Page: 0, PageSize: 30}, 0, 0, false, false},
	}

	for _, c := range cases {
		t.Run(c.name, func(t *testing.T) {
			got := buildPaginatedList(c.page, []int{}, c.total)
			if got.TotalPages != c.wantPages {
				t.Errorf("totalPages = %d, want %d", got.TotalPages, c.wantPages)
			}
			if got.TotalCount != c.total {
				t.Errorf("totalCount = %d, want %d", got.TotalCount, c.total)
			}
			if got.HasNextPage != c.wantNext {
				t.Errorf("hasNextPage = %v, want %v", got.HasNextPage, c.wantNext)
			}
			if got.HasPreviousPage != c.wantPrev {
				t.Errorf("hasPreviousPage = %v, want %v", got.HasPreviousPage, c.wantPrev)
			}
			if got.PageNumber != c.page.Page+1 {
				t.Errorf("pageNumber = %d, want %d (one-based)", got.PageNumber, c.page.Page+1)
			}
		})
	}
}

// TestBuildPaginatedListEmitsEmptyArray guards the one place Go's JSON encoder
// would silently differ from System.Text.Json: a nil slice marshals as null,
// while an empty List<T> marshals as []. The frontend calls .map and .filter on
// items directly, so null would crash the gallery.
func TestBuildPaginatedListEmitsEmptyArray(t *testing.T) {
	var missing []TagDto
	got := buildPaginatedList(pageParams{Page: 0, PageSize: 30}, missing, 0)

	encoded, err := jsonMarshal(got)
	if err != nil {
		t.Fatalf("marshal: %v", err)
	}
	if !contains(string(encoded), `"items":[]`) {
		t.Errorf("empty items should encode as [], got %s", encoded)
	}
}

func TestParseOptionalBoolIsStrict(t *testing.T) {
	// "yes" must be rejected rather than treated as false, otherwise a client
	// asking for NSFW-only results would silently receive everything.
	for _, target := range []string{"/images?isNsfw=yes", "/images?isNsfw=1x", "/images?isNsfw=on"} {
		r := httptest.NewRequest("GET", target, nil)
		if _, err := parseOptionalBool(r, "isNsfw"); err == nil {
			t.Errorf("%s should have been rejected", target)
		}
	}

	for _, target := range []string{"/images?isNsfw=true", "/images?isNsfw=false", "/images?isNsfw=1", "/images?isNsfw=0"} {
		r := httptest.NewRequest("GET", target, nil)
		value, err := parseOptionalBool(r, "isNsfw")
		if err != nil {
			t.Errorf("%s: %v", target, err)
			continue
		}
		if value == nil {
			t.Errorf("%s should have produced a value", target)
		}
	}
}

func TestParseIntListAcceptsBothForms(t *testing.T) {
	// Clients use both repeated parameters and comma-separated values.
	cases := map[string][]int64{
		"/images?includedIds=1&includedIds=2": {1, 2},
		"/images?includedIds=1,2":             {1, 2},
		"/images?includedIds=1,%202%20,3":     {1, 2, 3},
		"/images":                             nil,
	}

	for target, want := range cases {
		r := httptest.NewRequest("GET", target, nil)
		got := parseIntList(r, "includedIds")
		if len(got) != len(want) {
			t.Errorf("%s: got %v, want %v", target, got, want)
			continue
		}
		for i := range want {
			if got[i] != want[i] {
				t.Errorf("%s: got %v, want %v", target, got, want)
				break
			}
		}
	}
}

func TestStringListParamKeepsNonNumericValues(t *testing.T) {
	// A tag filter accepts either an id or a slug, so the values must survive as
	// strings; parsing here would lose the slugs.
	r := httptest.NewRequest("GET", "/images?includedTags=123,hatsune-miku&excludedTags=maid", nil)

	got := stringListParam(r.URL.Query(), "includedTags")
	want := []string{"123", "hatsune-miku"}
	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}

	if excluded := stringListParam(r.URL.Query(), "excludedTags"); len(excluded) != 1 || excluded[0] != "maid" {
		t.Errorf("excludedTags = %v, want [maid]", excluded)
	}
}

func TestNormalizeBasePathIsAppliedToRoutes(t *testing.T) {
	// http.ServeMux requires every pattern to begin with "/", so the conversion
	// has to happen before registration.
	cases := map[string]string{
		"":     "/",
		"/":    "/",
		"/api": "/api",
	}
	for configured, want := range cases {
		if got := basePathOrRoot(configured); got != want {
			t.Errorf("basePathOrRoot(%q) = %q, want %q", configured, got, want)
		}
	}
}
