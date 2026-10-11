package httpapi

import (
	"fmt"
	"net/http"
	"net/url"
	"strconv"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/config"
)

// PaginatedList is the envelope every list endpoint returns.
//
// This mirrors PaginatedList<T> in the C# service field for field, including the
// property order, because System.Text.Json emits them in declaration order and
// the contract is the bytes. The frontend reads data.items, data.pageNumber,
// data.totalCount and data.totalPages, so renaming or nesting any of these
// breaks pagination in the UI.
type PaginatedList struct {
	Items           any  `json:"items"`
	PageNumber      int  `json:"pageNumber"`
	TotalPages      int  `json:"totalPages"`
	TotalCount      int  `json:"totalCount"`
	MaxPageSize     int  `json:"maxPageSize"`
	DefaultPageSize int  `json:"defaultPageSize"`
	HasPreviousPage bool `json:"hasPreviousPage"`
	HasNextPage     bool `json:"hasNextPage"`
}

// pageParams holds the resolved pagination values.
type pageParams struct {
	// Page is zero-based internally; the API is one-based and the envelope
	// reports PageNumber one-based again.
	Page int

	// PageSize is the effective size after clamping.
	PageSize int

	// Limits are echoed back in the envelope, so the client can render the
	// pager without knowing the server configuration.
	Limits config.PageSize
}

// offset returns the SQL OFFSET for the page.
func (p pageParams) offset() int {
	return p.Page * p.PageSize
}

// pageNumber returns the one-based page number for the envelope.
func (p pageParams) pageNumber() int { return p.Page + 1 }

// parsePagination reads page and pageSize from the query string and normalises
// them the way PaginationUtils.Normalize does.
//
// Normalisation is deliberately permissive: a non-positive pageSize falls back
// to the configured default and is then clamped to the configured maximum, and
// anything below page 1 becomes page 1. The C# service does exactly this, so
// pageSize=0, pageSize=-1 and page=0 all behave the same way in both. A value
// that is not an integer at all is still rejected, because silently ignoring it
// would return a different result set than the client asked for.
func parsePagination(r *http.Request, limits config.PageSize) (pageParams, error) {
	query := r.URL.Query()

	page := 1
	if raw := query.Get("page"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return pageParams{}, apierror.BadRequest("The value 'page' is not valid.")
		}
		page = value
	}

	pageSize := limits.Default
	if raw := query.Get("pageSize"); raw != "" {
		value, err := strconv.Atoi(raw)
		if err != nil {
			return pageParams{}, apierror.BadRequest("The value 'pageSize' is not valid.")
		}
		pageSize = value
	}

	if pageSize <= 0 {
		pageSize = limits.Default
	}
	if limits.Max > 0 && pageSize > limits.Max {
		pageSize = limits.Max
	}
	if page < 1 {
		page = 1
	}

	return pageParams{
		Page:     page - 1,
		PageSize: pageSize,
		Limits:   limits,
	}, nil
}

// buildPaginatedList assembles the response envelope for a page of results.
//
// items must already be a non-nil slice: a nil slice marshals as null in Go,
// while the C# service always emits [] for an empty result, and the frontend
// calls .map and .filter on it directly.
func buildPaginatedList[T any](p pageParams, items []T, total int) PaginatedList {
	if items == nil {
		items = []T{}
	}

	totalPages := 0
	if p.PageSize > 0 {
		// Integer division rounded up, which is what Math.Ceiling does in C# for
		// the non-negative counts here.
		totalPages = (total + p.PageSize - 1) / p.PageSize
	}

	pageNumber := p.pageNumber()

	return PaginatedList{
		Items:           items,
		PageNumber:      pageNumber,
		TotalPages:      totalPages,
		TotalCount:      total,
		MaxPageSize:     p.Limits.Max,
		DefaultPageSize: p.Limits.Default,
		HasPreviousPage: pageNumber > 1,
		HasNextPage:     pageNumber < totalPages,
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

// stringListParam reads a repeated or comma-separated list parameter.
//
// Both "includedTags=a&includedTags=b" and "includedTags=a,b" are accepted,
// because clients differ and the C# service accepts both.
//
// Unlike parseIntList the values are kept as strings: a tag filter accepts
// either an id or a slug and the repository decides which, so parsing here would
// lose information.
func stringListParam(query url.Values, name string) []string {
	var values []string
	for _, raw := range query[name] {
		values = append(values, splitAndTrim(raw)...)
	}
	return values
}

// truthy interprets a boolean query parameter.
//
// ASP.NET Core's bool model binder accepts "true"/"false" case-insensitively
// and treats everything else as false, so this matches: an unrecognised value is
// false rather than a 400, which is what the frontend relies on when it sends
// nothing at all.
func truthy(value string) bool {
	return strings.EqualFold(strings.TrimSpace(value), "true")
}

// parsePositiveInt64 reads a strictly positive integer filter value.
//
// Zero is rejected because every caller uses it as an id, and id 0 does not
// exist in any of the tables these filters address.
func parsePositiveInt64(value string) (int64, error) {
	parsed, err := strconv.ParseInt(strings.TrimSpace(value), 10, 64)
	if err != nil {
		return 0, err
	}
	if parsed <= 0 {
		return 0, fmt.Errorf("value must be positive, got %d", parsed)
	}
	return parsed, nil
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
