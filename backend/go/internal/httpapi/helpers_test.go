package httpapi_test

import (
	"bytes"
	"fmt"
	"io"
	"net/http"
	"strconv"
	"strings"
	"testing"
)

// Small helpers shared by the integration tests. They exist so the tests read
// as a sequence of assertions rather than as JSON plumbing.

// readAll reads and closes a response body.
//
// A response body can only be read once, so every assertion that needs the body
// goes through here and works on the returned bytes. Reading it a second time
// would silently yield nothing and turn an assertion into a false pass.
func readAll(t *testing.T, resp *http.Response) []byte {
	t.Helper()

	defer resp.Body.Close()
	data, err := io.ReadAll(resp.Body)
	if err != nil {
		t.Fatalf("read response: %v", err)
	}
	return data
}

// jsonString extracts a string field from an already-read JSON body.
func jsonString(t *testing.T, body []byte, field string) string {
	t.Helper()

	needle := []byte(`"` + field + `":"`)
	index := bytes.Index(body, needle)
	if index < 0 {
		return ""
	}
	rest := body[index+len(needle):]
	end := bytes.IndexByte(rest, '"')
	if end < 0 {
		return ""
	}
	return string(rest[:end])
}

// jsonBool extracts a boolean field from an already-read JSON body.
func jsonBool(t *testing.T, body []byte, field string) bool {
	t.Helper()

	needle := []byte(`"` + field + `":`)
	index := bytes.Index(body, needle)
	if index < 0 {
		t.Fatalf("field %q not found in %s", field, string(body))
	}

	rest := body[index+len(needle):]
	if len(rest) > 0 && rest[0] == 't' {
		return true
	}
	return false
}

// jsonInt extracts an integer field from an already-read JSON body.
//
// It reports an error when the field is absent, because a missing field usually
// means the response was an error document rather than the expected payload.
func jsonInt(t *testing.T, body []byte, field string) (int, error) {
	needle := []byte(`"` + field + `":`)
	index := bytes.Index(body, needle)
	if index < 0 {
		return 0, fmt.Errorf("field %q not found in %s", field, string(body))
	}
	rest := body[index+len(needle):]

	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	return strconv.Atoi(string(rest[:end]))
}

// bytesReader wraps a string as a request body.
func bytesReader(body string) io.Reader {
	return strings.NewReader(body)
}

// itoa renders an int for use in paths and comparisons.
func itoa(v int) string { return strconv.Itoa(v) }

// containsBytes reports whether haystack contains needle.
func containsBytes(haystack []byte, needle string) bool {
	return bytes.Contains(haystack, []byte(needle))
}

// scanFirstID extracts the first `"id":N` from a JSON array of tasks.
func scanFirstID(body []byte, target *int) error {
	needle := []byte(`"id":`)
	index := bytes.Index(body, needle)
	if index < 0 {
		return fmt.Errorf("no id field in %s", string(body))
	}
	rest := body[index+len(needle):]

	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	value, err := strconv.Atoi(string(rest[:end]))
	if err != nil {
		return err
	}
	*target = value
	return nil
}

// scanTotal extracts the `"total":N` member from a paginated response.
func scanTotal(body []byte) int {
	needle := []byte(`"total":`)
	index := bytes.Index(body, needle)
	if index < 0 {
		return -1
	}
	rest := body[index+len(needle):]

	end := 0
	for end < len(rest) && rest[end] >= '0' && rest[end] <= '9' {
		end++
	}
	value, err := strconv.Atoi(string(rest[:end]))
	if err != nil {
		return -1
	}
	return value
}

// storageKeyFor mirrors the key format the storage layer writes.
func storageKeyFor(id int64, extension string) string {
	return fmt.Sprintf("%d%s", id, extension)
}
