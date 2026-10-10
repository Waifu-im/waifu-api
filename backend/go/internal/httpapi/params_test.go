package httpapi

import (
	"net/http/httptest"
	"testing"
)

func TestParsePaginationDefaults(t *testing.T) {
	r := httptest.NewRequest("GET", "/images", nil)

	page, err := parsePagination(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.Page != 0 {
		t.Errorf("default page = %d, want 0 (first page is index 0 internally)", page.Page)
	}
	if page.PageSize != defaultPageSize {
		t.Errorf("default page size = %d, want %d", page.PageSize, defaultPageSize)
	}
	if page.offset() != 0 {
		t.Errorf("offset of first page = %d, want 0", page.offset())
	}
}

func TestParsePaginationConvertsOneBasedPage(t *testing.T) {
	// The API is one-based because clients expect it, but SQL OFFSET is
	// zero-based. Getting this wrong silently returns the wrong page.
	r := httptest.NewRequest("GET", "/images?page=3&pageSize=20", nil)

	page, err := parsePagination(r)
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

func TestParsePaginationClampsUnlimited(t *testing.T) {
	// -1 means "no limit" in the C# service. Honouring it literally would let a
	// single request allocate an unbounded result set on a 512 MB instance.
	r := httptest.NewRequest("GET", "/images?pageSize=-1", nil)

	page, err := parsePagination(r)
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if page.PageSize != maxPageSize {
		t.Errorf("pageSize=-1 should clamp to %d, got %d", maxPageSize, page.PageSize)
	}

	r = httptest.NewRequest("GET", "/images?pageSize=100000", nil)
	page, _ = parsePagination(r)
	if page.PageSize != maxPageSize {
		t.Errorf("oversized pageSize should clamp to %d, got %d", maxPageSize, page.PageSize)
	}
}

func TestParsePaginationRejectsBadInput(t *testing.T) {
	cases := []string{
		"/images?page=0",
		"/images?page=-1",
		"/images?page=abc",
		"/images?pageSize=0",
		"/images?pageSize=abc",
	}
	for _, target := range cases {
		r := httptest.NewRequest("GET", target, nil)
		if _, err := parsePagination(r); err == nil {
			t.Errorf("%s should have been rejected", target)
		}
	}
}

func TestBuildPagination(t *testing.T) {
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
			got := buildPagination(c.page, c.total)
			if got.TotalPages != c.wantPages {
				t.Errorf("totalPages = %d, want %d", got.TotalPages, c.wantPages)
			}
			if got.Total != c.total {
				t.Errorf("total = %d, want %d", got.Total, c.total)
			}
			if got.HasNextPage != c.wantNext {
				t.Errorf("hasNextPage = %v, want %v", got.HasNextPage, c.wantNext)
			}
			if got.HasPrevPage != c.wantPrev {
				t.Errorf("hasPrevPage = %v, want %v", got.HasPrevPage, c.wantPrev)
			}
			if got.CurrentPage != c.page.Page+1 {
				t.Errorf("currentPage = %d, want %d (one-based)", got.CurrentPage, c.page.Page+1)
			}
		})
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

func TestParseOptionalBoolDistinguishesAbsentFromFalse(t *testing.T) {
	// Omitting the filter and explicitly asking for false are different queries.
	// Conflating them would change which images come back.
	r := httptest.NewRequest("GET", "/images", nil)
	value, err := parseOptionalBool(r, "isNsfw")
	if err != nil {
		t.Fatalf("unexpected error: %v", err)
	}
	if value != nil {
		t.Errorf("absent filter should be nil, got %v", *value)
	}

	r = httptest.NewRequest("GET", "/images?isNsfw=false", nil)
	value, _ = parseOptionalBool(r, "isNsfw")
	if value == nil || *value != false {
		t.Errorf("explicit false should be a non-nil false, got %v", value)
	}
}

func TestParseIntListAcceptsBothForms(t *testing.T) {
	// Clients use both repeated parameters and comma-separated values.
	cases := map[string][]int64{
		"/images?excluded=1&excluded=2": {1, 2},
		"/images?excluded=1,2":          {1, 2},
		"/images?excluded=1,%202%20,3":  {1, 2, 3},
		"/images":                       nil,
	}

	for target, want := range cases {
		r := httptest.NewRequest("GET", target, nil)
		got := parseIntList(r, "excluded")
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

func TestParseStringList(t *testing.T) {
	r := httptest.NewRequest("GET", "/images?tags=hatsune-miku,vocaloid&tags=maid", nil)
	got := parseStringList(r, "tags")
	want := []string{"hatsune-miku", "vocaloid", "maid"}

	if len(got) != len(want) {
		t.Fatalf("got %v, want %v", got, want)
	}
	for i := range want {
		if got[i] != want[i] {
			t.Fatalf("got %v, want %v", got, want)
		}
	}
}

func TestNormalizeBasePathIsAppliedToRoutes(t *testing.T) {
	// Covered by config tests, but the interaction with route patterns matters:
	// http.ServeMux requires every pattern to begin with "/".
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
