package httpapi_test

import (
	"encoding/json"
	"fmt"
	"image/color"
	"net/http"
	"strings"
	"testing"

	"waifuapi/internal/config"
)

// Coverage for the artist read endpoints, which had none.
//
// GET /artists and GET /artists/by-name/{name} were both at 0%: no test ever
// called them. Listing endpoints are where the quiet bugs live, because a
// broken sort or a dropped image count still returns 200 with a plausible body.

func seedArtists(t *testing.T, h *harness, names ...string) []int64 {
	t.Helper()
	ids := make([]int64, 0, len(names))
	for _, name := range names {
		token := h.issueToken(1, config.RoleTrustedUser)
		payload := fmt.Sprintf(`{"name":%q,"description":"about %s","patreon":"https://patreon.com/%s"}`,
			name, name, name)
		resp := h.request(http.MethodPost, "/artists", token, strings.NewReader(payload), "application/json")
		if resp.StatusCode != http.StatusCreated {
			t.Fatalf("seed artist %q: status %d: %s", name, resp.StatusCode, readAll(t, resp))
		}
		id, err := jsonInt(t, readAll(t, resp), "id")
		resp.Body.Close()
		if err != nil {
			t.Fatalf("seed artist %q: %v", name, err)
		}
		ids = append(ids, int64(id))
	}
	return ids
}

func TestListArtistsReturnsAnEnvelope(t *testing.T) {
	h := newHarness(t)
	seedArtists(t, h, "alpha", "beta", "gamma")

	resp := h.request(http.MethodGet, "/artists", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
	}
	payload := readAll(t, resp)

	for _, field := range []string{"items", "totalCount", "pageNumber", "totalPages"} {
		if !jsonHas(payload, field) {
			t.Errorf("envelope is missing %q", field)
		}
	}

	if total := jsonIntOrZero(t, payload, "totalCount"); total < 3 {
		t.Errorf("totalCount = %d, want at least the three seeded artists", total)
	}
}

func TestListArtistsIsPublic(t *testing.T) {
	// The gallery links to an artist page, so this must not require a token.
	h := newHarness(t)
	resp := h.request(http.MethodGet, "/artists", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Errorf("status = %d, want 200 without a token", resp.StatusCode)
	}
}

func TestListArtistsIncludesImageCount(t *testing.T) {
	h := newHarness(t)
	ids := seedArtists(t, h, "counted")

	// Attach two accepted images to the artist. They must differ in pixels:
	// two identical solid fills share a perceptual hash and the second upload
	// is rejected as a duplicate, which is the dedupe working as intended.
	attachArtist(t, h, ids[0], "artist-image-a", 100, 100, color.RGBA{90, 90, 140, 255})
	attachArtist(t, h, ids[0], "artist-image-b", 140, 100, color.RGBA{200, 60, 60, 255})

	resp := h.request(http.MethodGet, "/artists", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		t.Fatalf("status = %d: %s", resp.StatusCode, readAll(t, resp))
	}

	// Locate the artist we seeded rather than assuming it is first; the harness
	// may already have rows in the table.
	entries, err := decodeArtistItems(readAll(t, resp))
	if err != nil {
		t.Fatalf("decode artists: %v", err)
	}

	var found bool
	for _, artist := range entries {
		if artist.ID != ids[0] {
			continue
		}
		found = true
		// The count is what the artist card renders, so it has to be correct
		// rather than merely non-erroring.
		if got := artist.ImageCount; got != 2 {
			t.Errorf("imageCount = %d, want 2", got)
		}
	}
	if !found {
		t.Fatalf("seeded artist %d is not present in the listing", ids[0])
	}
}

type artistItem struct {
	ID         int64  `json:"id"`
	Name       string `json:"name"`
	ImageCount int    `json:"imageCount"`
}

func decodeArtistItems(payload []byte) ([]artistItem, error) {
	var envelope struct {
		Items []artistItem `json:"items"`
	}
	err := json.Unmarshal(payload, &envelope)
	return envelope.Items, err
}

func TestGetArtistByNameIsCaseInsensitive(t *testing.T) {
	h := newHarness(t)
	seedArtists(t, h, "SomeArtist")

	for _, path := range []string{"/artists/by-name/someartist", "/artists/by-name/SOMEARTIST"} {
		t.Run(path, func(t *testing.T) {
			resp := h.request(http.MethodGet, path, "", nil, "")
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d for %s: %s", resp.StatusCode, path, readAll(t, resp))
			}
		})
	}
}

func TestGetArtistByNameNotFound(t *testing.T) {
	h := newHarness(t)

	resp := h.request(http.MethodGet, "/artists/by-name/nobody", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound {
		t.Errorf("status = %d, want 404 for an unknown artist", resp.StatusCode)
	}
}

func TestGetArtistByNameEmptyNameIsNotFound(t *testing.T) {
	h := newHarness(t)

	// An empty name must not resolve to some arbitrary artist.
	resp := h.request(http.MethodGet, "/artists/by-name/%20", "", nil, "")
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusNotFound && resp.StatusCode != http.StatusBadRequest {
		t.Errorf("status = %d, want 404 or 400 for a blank name", resp.StatusCode)
	}
}

func TestListArtistsNormalisesPagination(t *testing.T) {
	h := newHarness(t)
	seedArtists(t, h, "paged")

	for _, query := range []string{"?page=0", "?pageSize=0", "?pageSize=-3"} {
		t.Run(query, func(t *testing.T) {
			resp := h.request(http.MethodGet, "/artists"+query, "", nil, "")
			defer resp.Body.Close()

			if resp.StatusCode != http.StatusOK {
				t.Fatalf("status = %d for %s: %s", resp.StatusCode, query, readAll(t, resp))
			}
			if page := jsonIntOrZero(t, readAll(t, resp), "pageNumber"); page < 1 {
				t.Errorf("pageNumber = %d, want a normalised value of at least 1", page)
			}
		})
	}
}

// attachArtist links one accepted image to an artist via the upload form.
func attachArtist(t *testing.T, h *harness, artistID int64, slug string, w, h2 int, fill color.RGBA) {
	t.Helper()

	token := h.issueToken(2, config.RoleAdmin)

	body, contentType := multipartImage(t, "file", slug+".png", testImage(t, w, h2, fill), map[string]string{
		"artists": fmt.Sprintf("%d", artistID),
	})
	resp := h.request(http.MethodPost, "/images/upload", token, body, contentType)
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusCreated {
		t.Fatalf("attach image for artist: status %d: %s", resp.StatusCode, readAll(t, resp))
	}
}

// jsonItemIntOrZero reads a numeric field from the first element of items.
func jsonItemIntOrZero(payload []byte, field string) int {
	var envelope struct {
		Items []map[string]int `json:"items"`
	}
	if err := json.Unmarshal(payload, &envelope); err != nil {
		return 0
	}
	if len(envelope.Items) == 0 {
		return 0
	}
	return envelope.Items[0][field]
}
