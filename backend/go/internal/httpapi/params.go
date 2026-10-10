package httpapi

import (
	"net/http"
	"strconv"
	"strings"

	"waifuapi/internal/apierror"
)

// Pagination is the envelope every list endpoint returns.
//
// The field names are the wire contract. The frontend's useResource hook reads
// items, total, currentPage and totalPages, so renaming any of them breaks
// pagination in the UI.
type Pagination struct {
	CurrentPage int  `json:"currentPage"`
	TotalPages  int  `json:"totalPages"`
	Total       int  `json:"total"`
	HasNextPage bool `json:"hasNextPage"`
	HasPrevPage bool `json:"hasPrevPage"`
}

// Default and maximum page sizes.
//
// The C# service defaults to 30 and allows -1 for "no limit". The limit is kept
// here because an unbounded page over a table with thousands of rows is an easy
// way to exhaust memory on a 512 MB instance.
const (
	defaultPageSize = 30
	maxPageSize     = 200
)

// pageParams holds the resolved pagination values.
type pageParams struct {
	Page     int
	PageSize int
}

// offset returns the SQL OFFSET for the page.
func (p pageParams) offset() int {
	return p.Page * p.PageSize
}

// parsePagination reads page and pageSize from the query string.
//
// page is one-based in the API and zero-based internally, which keeps the SQL
// offset trivial while matching what clients expect.
func parsePagination(r *http.Request) (pageParams, error) {
	query := r.URL.Query()

	page := 0
	if raw := query.Get("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return pageParams{}, apierror.BadRequest("'page' must be an integer.")
		}
		if value < 1 {
			return pageParams{}, apierror.BadRequest("'page' must be 1 or greater.")
		}
		page = value - 1
	}

	pageSize := defaultPageSize
	if raw := query.Get("pageSize"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return pageParams{}, apierror.BadRequest("'pageSize' must be an integer.")
		}
		// -1 means "no limit" in the C# service. It is honoured, but clamped so
		// a single request cannot allocate an unbounded result.
		if value == -1 || value > maxPageSize {
			value = maxPageSize
		}
		if value < 1 {
			return pageParams{}, apierror.BadRequest("'pageSize' must be 1 or greater.")
		}
		pageSize = value
	}

	return pageParams{Page: page, PageSize: pageSize}, nil
}

// buildPagination computes the envelope for a page of results.
func buildPagination(p pageParams, total int) Pagination {
	totalPages := 0
	if p.PageSize > 0 {
		totalPages = (total + p.PageSize - 1) / p.PageSize
	}

	return Pagination{
		CurrentPage: p.Page + 1,
		TotalPages:  totalPages,
		Total:       total,
		HasNextPage: p.Page+1 < totalPages,
		HasPrevPage: p.Page > 0,
	}
}

// parseOptionalInt reads an optional integer query parameter.
//
// A malformed value is an error rather than being ignored, because silently
// dropping a filter would return a wider result set than the client asked for,
// which looks like a bug in the API rather than in the request.
func parseOptionalInt(r *http.Request, name string) (*int, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.Atoi(raw)
	if err != nil {
		return nil, apierror.BadRequest("'" + name + "' must be an integer.")
	}
	return &value, nil
}

// parseOptionalInt64 is parseOptionalInt for 64-bit values such as file sizes.
func parseOptionalInt64(r *http.Request, name string) (*int64, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseInt(raw, 10, 64)
	if err != nil {
		return nil, apierror.BadRequest("'" + name + "' must be an integer.")
	}
	return &value, nil
}

// parseOptionalBool reads an optional boolean query parameter.
//
// Only the literal forms strconv accepts are allowed, so "yes" is rejected
// rather than being silently treated as false.
func parseOptionalBool(r *http.Request, name string) (*bool, error) {
	raw := r.URL.Query().Get(name)
	if raw == "" {
		return nil, nil
	}
	value, err := strconv.ParseBool(raw)
	if err != nil {
		return nil, apierror.BadRequest("'" + name + "' must be true or false.")
	}
	return &value, nil
}

// parseIntList reads a repeated or comma-separated list parameter.
//
// Both "ids=1&ids=2" and "ids=1,2" are accepted because clients differ, and the
// C# service accepts both.
func parseIntList(r *http.Request, name string) []int64 {
	var values []int64

	for _, raw := range r.URL.Query()[name] {
		for _, part := range splitAndTrim(raw) {
			value, err := strconv.ParseInt(part, 10, 64)
			if err != nil {
				continue
			}
			values = append(values, value)
		}
	}
	return values
}

// parseStringList reads a repeated or comma-separated string parameter.
func parseStringList(r *http.Request, name string) []string {
	var values []string

	for _, raw := range r.URL.Query()[name] {
		values = append(values, splitAndTrim(raw)...)
	}
	return values
}

// splitAndTrim splits on commas and drops empty entries.
func splitAndTrim(raw string) []string {
	var out []string
	for _, part := range strings.Split(raw, ",") {
		trimmed := strings.TrimSpace(part)
		if trimmed != "" {
			out = append(out, trimmed)
		}
	}
	return out
}

// basePathOrRoot returns a base path suitable for ServeMux patterns.
//
// net/http requires every pattern to start with "/", so an empty configured
// base path becomes "/" rather than "".
func basePathOrRoot(basePath string) string {
	if basePath == "" {
		return "/"
	}
	return basePath
}
