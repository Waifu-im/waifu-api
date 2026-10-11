package httpapi_test

import (
	"context"
	"encoding/json"
	"net/http"
	"net/url"
	"strings"
	"testing"
	"time"

	"waifuapi/internal/config"
)

// GET /images is the highest-traffic endpoint in the service and its query was
// rewritten wholesale during the parity work: the filter parameter names, the
// tri-state enums, the range expressions and the pagination envelope all differ
// from the first port. These tests pin the C# behaviour, because a filter that
// is silently ignored looks exactly like "the API returned nothing".

// seededImage describes one row to insert directly, bypassing the upload path so
// a filter test is not also a test of the uploader.
type seededImage struct {
	Width, Height, ByteSize int64
	Extension               string
	IsNsfw, IsAnimated      bool
	ReviewStatus            int
	UploadedAt              time.Time
	Tags                    []string
	Artists                 []int64
	UploaderID              *int64
}

func (s seededImage) insert(t *testing.T, h *harness) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	uploaded := s.UploadedAt
	if uploaded.IsZero() {
		uploaded = time.Now().UTC()
	}

	var id int64
	err := h.db.Pool().QueryRow(ctx, `
        INSERT INTO "Images" ("Width","Height","ByteSize","Extension","DominantColor",
                              "IsNsfw","IsAnimated","UploadedAt","PerceptualHash","ReviewStatus","UploaderId")
        VALUES ($1,$2,$3,$4,'#000000',$5,$6,$7,repeat('0',64)::bit(64),$8,$9)
        RETURNING "Id"`,
		s.Width, s.Height, s.ByteSize, s.Extension, s.IsNsfw, s.IsAnimated,
		uploaded, s.ReviewStatus, s.UploaderID).Scan(&id)
	if err != nil {
		t.Fatalf("seed image: %v", err)
	}

	for _, slug := range s.Tags {
		tagID := ensureTag(t, h, slug)
		if _, err := h.db.Pool().Exec(ctx,
			`INSERT INTO "ImageTag" ("ImagesId","TagsId") VALUES ($1,$2)`, id, tagID); err != nil {
			t.Fatalf("link tag: %v", err)
		}
	}
	for _, artistID := range s.Artists {
		if _, err := h.db.Pool().Exec(ctx,
			`INSERT INTO "ArtistImage" ("ImagesId","ArtistsId") VALUES ($1,$2)`, id, artistID); err != nil {
			t.Fatalf("link artist: %v", err)
		}
	}
	return id
}

// ensureTag creates a tag if absent and returns its id.
func ensureTag(t *testing.T, h *harness, slug string) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id int64
	err := h.db.Pool().QueryRow(ctx, `
        INSERT INTO "Tags" ("Name","Slug","Description","ReviewStatus")
        VALUES ($1,$1,'',1)
        ON CONFLICT DO NOTHING
        RETURNING "Id"`, slug).Scan(&id)
	if err != nil {
		var existing int64
		if err2 := h.db.Pool().QueryRow(ctx,
			`SELECT "Id" FROM "Tags" WHERE "Slug" = $1`, slug).Scan(&existing); err2 != nil {
			t.Fatalf("ensure tag: %v", err)
		}
		return existing
	}
	return id
}

// ensureArtist creates an artist if absent and returns its id.
func ensureArtist(t *testing.T, h *harness, name string) int64 {
	t.Helper()

	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()

	var id int64
	err := h.db.Pool().QueryRow(ctx, `
        INSERT INTO "Artists" ("Name","ReviewStatus") VALUES ($1,1)
        ON CONFLICT DO NOTHING
        RETURNING "Id"`, name).Scan(&id)
	if err != nil {
		var existing int64
		if err2 := h.db.Pool().QueryRow(ctx,
			`SELECT "Id" FROM "Artists" WHERE "Name" = $1`, name).Scan(&existing); err2 != nil {
			t.Fatalf("ensure artist: %v", err)
		}
		return existing
	}
	return id
}

// imageIDs reads the ids out of a gallery response, in the order returned.
func imageIDs(t *testing.T, body []byte) []int64 {
	t.Helper()

	var envelope struct {
		Items []struct {
			ID int64 `json:"id"`
		} `json:"items"`
		TotalCount int `json:"totalCount"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode gallery response: %v (%s)", err, body)
	}

	ids := make([]int64, 0, len(envelope.Items))
	for _, item := range envelope.Items {
		ids = append(ids, item.ID)
	}
	return ids
}

// totalCount reads the envelope's total.
func totalCount(t *testing.T, body []byte) int {
	t.Helper()

	var envelope struct {
		TotalCount int `json:"totalCount"`
	}
	if err := json.Unmarshal(body, &envelope); err != nil {
		t.Fatalf("decode envelope: %v (%s)", err, body)
	}
	return envelope.TotalCount
}

// gallery runs GET /images with a query and returns the body.
func (h *harness) gallery(t *testing.T, token, query string) []byte {
	t.Helper()

	target := "/images"
	if query != "" {
		target += "?" + query
	}
	resp := h.request("GET", target, token, nil, "")
	if resp.StatusCode != http.StatusOK {
		t.Fatalf("GET %s = %d: %s", target, resp.StatusCode, readAll(t, resp))
	}
	return readAll(t, resp)
}

// assertSameIDs compares two id sets, ignoring order where the sort is random.
func assertSameIDs(t *testing.T, got []int64, want []int64, label string) {
	t.Helper()

	if len(got) != len(want) {
		t.Errorf("%s: got %v, want %v", label, got, want)
		return
	}
	seen := map[int64]bool{}
	for _, id := range want {
		seen[id] = true
	}
	for _, id := range got {
		if !seen[id] {
			t.Errorf("%s: got %v, want %v", label, got, want)
			return
		}
	}
}

// TestGalleryEnvelopeShape pins the PaginatedList<T> contract.
//
// The first port emitted {"result": [...], "pagination": {...}}, and the frontend
// reads data.items — so every list endpoint was returning a shape the UI could
// not read. The field names here are the wire contract.
func TestGalleryEnvelopeShape(t *testing.T) {
	h := newHarness(t)
	seededImage{Width: 100, Height: 100, ByteSize: 10, Extension: ".png", ReviewStatus: 1}.insert(t, h)

	body := h.gallery(t, "", "")

	for _, field := range []string{
		`"items"`, `"pageNumber"`, `"totalPages"`, `"totalCount"`,
		`"maxPageSize"`, `"defaultPageSize"`, `"hasPreviousPage"`, `"hasNextPage"`,
	} {
		if !strings.Contains(string(body), field) {
			t.Errorf("envelope is missing %s: %s", field, body)
		}
	}

	// The old invented envelope must be gone.
	for _, field := range []string{`"result"`, `"pagination"`, `"currentPage"`, `"totalPages":0,"total`} {
		if field == `"totalPages"` {
			continue
		}
		if strings.Contains(string(body), field) {
			t.Errorf("envelope still carries the invented field %s: %s", field, body)
		}
	}
}

// TestGalleryImageShape pins ImageDto.
//
// perceptualHash, reviewStatus, uploaderId, favorites, likedAt and albums are
// each read by the frontend; dropping one renders a broken page rather than an
// error.
func TestGalleryImageShape(t *testing.T) {
	h := newHarness(t)
	seededImage{Width: 100, Height: 100, ByteSize: 10, Extension: ".png", ReviewStatus: 1}.insert(t, h)

	body := h.gallery(t, "", "")

	for _, field := range []string{
		`"id"`, `"perceptualHash"`, `"extension"`, `"dominantColor"`, `"source"`,
		`"artists"`, `"uploaderId"`, `"uploadedAt"`, `"isNsfw"`, `"isAnimated"`,
		`"width"`, `"height"`, `"byteSize"`, `"url"`, `"tags"`, `"reviewStatus"`,
		`"favorites"`, `"likedAt"`, `"addedToAlbumAt"`, `"albums"`,
	} {
		if !strings.Contains(string(body), field) {
			t.Errorf("image is missing %s: %s", field, body)
		}
	}

	// uploader is an invented field the first port emitted instead of uploaderId.
	if strings.Contains(string(body), `"uploader"`) {
		t.Errorf("image should carry uploaderId, not uploader: %s", body)
	}
}

// TestGalleryDefaultsToSFWOnly is the single most consequential filter.
//
// NsfwMode defaults to False in C#, so an anonymous caller cannot reach NSFW
// content by omitting the parameter. Getting this wrong is a content-exposure
// bug, not a cosmetic one.
func TestGalleryDefaultsToSFWOnly(t *testing.T) {
	h := newHarness(t)

	sfw := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		IsNsfw: true, ReviewStatus: 1}.insert(t, h)

	body := h.gallery(t, "", "")
	assertSameIDs(t, imageIDs(t, body), []int64{sfw}, "default (SFW only)")
	if got := totalCount(t, body); got != 1 {
		t.Errorf("default totalCount = %d, want 1", got)
	}

	body = h.gallery(t, "", "isNsfw=true")
	if got := len(imageIDs(t, body)); got != 1 || totalCount(t, body) != 1 {
		t.Errorf("isNsfw=true should return only the NSFW image, got %s", body)
	}

	body = h.gallery(t, "", "isNsfw=all")
	if got := totalCount(t, body); got != 2 {
		t.Errorf("isNsfw=all totalCount = %d, want 2", got)
	}

	// The tri-state is a contract, not a boolean: "yes" must be refused rather
	// than silently treated as false.
	resp := h.request("GET", "/images?isNsfw=yes", "", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("isNsfw=yes should be rejected, got %d", resp.StatusCode)
	}
}

// TestGalleryHidesPendingFromThePublic is the review boundary.
func TestGalleryHidesPendingFromThePublic(t *testing.T) {
	h := newHarness(t)

	accepted := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	pending := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 0}.insert(t, h)

	body := h.gallery(t, "", "")
	assertSameIDs(t, imageIDs(t, body), []int64{accepted}, "anonymous")
	if got := totalCount(t, body); got != 1 {
		t.Errorf("anonymous totalCount = %d, want 1 (pending hidden)", got)
	}

	mod := h.issueToken(21, config.RoleModerator)
	body = h.gallery(t, mod, "reviewStatus=all")
	assertSameIDs(t, imageIDs(t, body), []int64{accepted, pending}, "moderator reviewStatus=all")

	// A non-moderator asking for pending content is refused with 403, not given
	// a silently-downgraded list.
	user := h.issueToken(22, config.RoleUser)
	resp := h.request("GET", "/images?reviewStatus=pending", user, nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusForbidden {
		t.Errorf("non-moderator reviewStatus=pending = %d, want 403", resp.StatusCode)
	}
}

// TestGalleryIncludeMyPending unions the caller's own uploads.
func TestGalleryIncludeMyPending(t *testing.T) {
	h := newHarness(t)

	// The harness seeds users 1..100; Image.UploaderId is a real foreign key.
	mine := int64(11)
	other := int64(12)

	accepted := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	minePending := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 0, UploaderID: &mine}.insert(t, h)
	otherPending := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 0, UploaderID: &other}.insert(t, h)

	_ = otherPending

	token := h.issueToken(mine, config.RoleUser)
	body := h.gallery(t, token, "includeMyPending=true")

	assertSameIDs(t, imageIDs(t, body),
		[]int64{accepted, minePending}, "includeMyPending shows only my pending")
}

// TestGalleryTagFilter pins the AND semantics.
//
// IncludedTags is AND across values and each value may be an id or a slug; the
// first port used a single "tags" parameter with OR semantics.
func TestGalleryTagFilter(t *testing.T) {
	h := newHarness(t)

	ensureTag(t, h, "waifu")
	ensureTag(t, h, "maid")

	both := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, Tags: []string{"waifu", "maid"}}.insert(t, h)
	onlyWaifu := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, Tags: []string{"waifu"}}.insert(t, h)

	body := h.gallery(t, "", "includedTags=waifu")
	assertSameIDs(t, imageIDs(t, body), []int64{both, onlyWaifu}, "includedTags=waifu")

	// Two values are ANDed, not ORed.
	body = h.gallery(t, "", "includedTags=waifu&includedTags=maid")
	assertSameIDs(t, imageIDs(t, body), []int64{both}, "includedTags AND semantics")

	body = h.gallery(t, "", "excludedTags=maid")
	assertSameIDs(t, imageIDs(t, body), []int64{onlyWaifu}, "excludedTags")

	// A tag id works wherever a slug does.
	waifuID := int64(0)
	ctx, cancel := context.WithTimeout(context.Background(), 10*time.Second)
	defer cancel()
	if err := h.db.Pool().QueryRow(ctx,
		`SELECT "Id" FROM "Tags" WHERE "Slug" = 'waifu'`).Scan(&waifuID); err != nil {
		t.Fatalf("read tag id: %v", err)
	}
	// A tag id is accepted wherever a slug is, because the filter treats an
	// integer value as an id and anything else as a slug.
	body = h.gallery(t, "", "includedTags="+itoa(int(waifuID)))
	assertSameIDs(t, imageIDs(t, body), []int64{both, onlyWaifu}, "includedTags by id")
}

// TestGalleryArtistFilter covers the id-or-name union.
//
// IncludedArtists accepts an id or a lowercased name; the first port read a single
// "artists" parameter and matched on id only.
func TestGalleryArtistFilter(t *testing.T) {
	h := newHarness(t)

	artistID := ensureArtist(t, h, "Miku")
	withArtist := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, Artists: []int64{artistID}}.insert(t, h)
	without := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1}.insert(t, h)

	body := h.gallery(t, "", "includedArtists="+itoa(int(artistID)))
	assertSameIDs(t, imageIDs(t, body), []int64{withArtist}, "includedArtists by id")

	// Names are matched case-insensitively.
	body = h.gallery(t, "", "includedArtists=miku")
	assertSameIDs(t, imageIDs(t, body), []int64{withArtist}, "includedArtists by name")

	body = h.gallery(t, "", "excludedArtists=miku")
	assertSameIDs(t, imageIDs(t, body), []int64{without}, "excludedArtists by name")
}

// TestGalleryOrientationFilter pins each orientation.
//
// Note the C# definitions: Landscape is Width > Height and Portrait is
// Height > Width, so they are the mirror of what the names suggest in a
// landscape-shaped image.
func TestGalleryOrientationFilter(t *testing.T) {
	h := newHarness(t)

	landscape := seededImage{Width: 200, Height: 100, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	portrait := seededImage{Width: 100, Height: 200, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	square := seededImage{Width: 150, Height: 150, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)

	body := h.gallery(t, "", "orientation=Landscape")
	assertSameIDs(t, imageIDs(t, body), []int64{landscape}, "landscape")

	body = h.gallery(t, "", "orientation=Portrait")
	assertSameIDs(t, imageIDs(t, body), []int64{portrait}, "portrait")

	body = h.gallery(t, "", "orientation=Square")
	assertSameIDs(t, imageIDs(t, body), []int64{square}, "square")

	body = h.gallery(t, "", "orientation=All")
	if got := len(imageIDs(t, body)); got != 3 {
		t.Errorf("orientation=All returned %d, want 3", got)
	}

	resp := h.request("GET", "/images?orientation=diagonal", "", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("unknown orientation should be rejected, got %d", resp.StatusCode)
	}
}

// TestGalleryRangeExpressions pins the five accepted forms.
//
// The C# service accepts an exact value, ">=x", ">x", "<=x", "<x" and "a..b".
// A malformed expression is silently ignored rather than rejected, so the test
// also proves the ignoring behaviour is deliberate rather than accidental.
func TestGalleryRangeExpressions(t *testing.T) {
	h := newHarness(t)

	small := seededImage{Width: 100, Height: 100, ByteSize: 1000, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	medium := seededImage{Width: 500, Height: 500, ByteSize: 5000, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	large := seededImage{Width: 2000, Height: 2000, ByteSize: 9000, Extension: ".png", ReviewStatus: 1}.insert(t, h)

	// The operator must be percent-encoded, which is what URLSearchParams does in
	// the frontend. Writing "width<=500" raw splits on the first "=" and produces
	// a parameter named "width<", which is a malformed request rather than a
	// range filter — and ASP.NET Core behaves identically, so this is the
	// contract rather than a Go quirk.
	//
	// The frontend's own placeholder is "Width (e.g. > 1920)", so a space after
	// the operator is the expected shape.
	cases := []struct {
		query string
		want  []int64
		label string
	}{
		{"width=500", []int64{medium}, "exact"},
		{"width=" + url.QueryEscape(">=500"), []int64{medium, large}, "greater or equal"},
		{"width=" + url.QueryEscape("> 1920"), []int64{large}, "greater with a space"},
		{"width=" + url.QueryEscape("<=500"), []int64{small, medium}, "less or equal"},
		{"width=" + url.QueryEscape("<500"), []int64{small}, "less"},
		{"width=100..500", []int64{small, medium}, "range"},
		{"byteSize=" + url.QueryEscape(">=5000"), []int64{medium, large}, "byteSize range"},
		{"height=2000", []int64{large}, "height exact"},
		// An unparseable expression is ignored, matching ApplyRangeFilter.
		{"width=notanumber", []int64{small, medium, large}, "malformed is ignored"},
	}

	for _, c := range cases {
		body := h.gallery(t, "", c.query)
		assertSameIDs(t, imageIDs(t, body), c.want, c.label)
	}
}

// TestGalleryIncludedAndExcludedIDs pins id-list filtering.
func TestGalleryIncludedAndExcludedIDs(t *testing.T) {
	h := newHarness(t)

	first := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	second := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)

	body := h.gallery(t, "", "includedIds="+itoa(int(first)))
	assertSameIDs(t, imageIDs(t, body), []int64{first}, "includedIds")

	body = h.gallery(t, "", "excludedIds="+itoa(int(first)))
	assertSameIDs(t, imageIDs(t, body), []int64{second}, "excludedIds")
}

// TestGallerySortOrder pins the three sort modes that are deterministic.
//
// Random is not asserted because it has no stable order to assert against; what
// matters is that it is accepted.
func TestGallerySortOrder(t *testing.T) {
	h := newHarness(t)

	older := time.Now().UTC().Add(-48 * time.Hour)
	newer := time.Now().UTC()

	first := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, UploadedAt: older}.insert(t, h)
	second := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png",
		ReviewStatus: 1, UploadedAt: newer}.insert(t, h)

	body := h.gallery(t, "", "orderBy=UploadedAt")
	if got := imageIDs(t, body); len(got) != 2 || got[0] != second {
		t.Errorf("orderBy=UploadedAt should be newest first, got %v", got)
	}

	// AddedToAlbum is meaningless outside an album and is refused.
	resp := h.request("GET", "/images?orderBy=AddedToAlbum", "", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusBadRequest {
		t.Errorf("orderBy=AddedToAlbum outside an album = %d, want 400", resp.StatusCode)
	}

	resp = h.request("GET", "/images?orderBy=Random", "", nil, "")
	defer resp.Body.Close()
	if resp.StatusCode != http.StatusOK {
		t.Errorf("orderBy=Random = %d, want 200", resp.StatusCode)
	}
	_ = first
}

// TestGalleryPagination covers the offset arithmetic end to end.
//
// page is one-based on the wire and zero-based in SQL; getting the conversion
// wrong returns the wrong page silently rather than erroring.
func TestGalleryPagination(t *testing.T) {
	h := newHarness(t)

	for i := 0; i < 5; i++ {
		seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	}

	page1 := imageIDs(t, h.gallery(t, "", "pageSize=2&page=1&orderBy=UploadedAt"))
	page2 := imageIDs(t, h.gallery(t, "", "pageSize=2&page=2&orderBy=UploadedAt"))
	page3 := imageIDs(t, h.gallery(t, "", "pageSize=2&page=3&orderBy=UploadedAt"))

	if len(page1) != 2 || len(page2) != 2 || len(page3) != 1 {
		t.Fatalf("page sizes = %d/%d/%d, want 2/2/1", len(page1), len(page2), len(page3))
	}

	seen := map[int64]bool{}
	for _, id := range append(append(page1, page2...), page3...) {
		if seen[id] {
			t.Errorf("image %d appeared on more than one page", id)
		}
		seen[id] = true
	}
	if len(seen) != 5 {
		t.Errorf("paging produced %d distinct images, want 5", len(seen))
	}

	// The envelope reports the full total, not the page size.
	if got := totalCount(t, h.gallery(t, "", "pageSize=2")); got != 5 {
		t.Errorf("totalCount = %d, want 5", got)
	}
}

// TestGalleryAlbumVariant covers the album-scoped query, which takes a different
// code path in the repository because it joins AlbumItem.
func TestGalleryAlbumVariant(t *testing.T) {
	h := newHarness(t)

	// The harness seeds users 1..100, and Album.UserId is a real foreign key, so
	// the caller has to be one of them.
	user := int64(7)
	token := h.issueToken(user, config.RoleUser)

	inAlbum := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)
	outOfAlbum := seededImage{Width: 10, Height: 10, ByteSize: 1, Extension: ".png", ReviewStatus: 1}.insert(t, h)

	resp := h.request("POST", "/users/me/albums", token,
		bytesReader(`{"name":"Trips"}`), "application/json")
	albumID, err := jsonInt(t, readAll(t, resp), "id")
	if err != nil {
		t.Fatalf("no album id: %v", err)
	}

	for _, id := range []int64{inAlbum, outOfAlbum} {
		resp := h.request("POST",
			"/users/me/albums/"+itoa(albumID)+"/images/"+itoa64(id), token, nil, "")
		if resp.StatusCode != http.StatusNoContent {
			t.Fatalf("add %d status = %d", id, resp.StatusCode)
		}
	}

	resp = h.request("DELETE",
		"/users/me/albums/"+itoa(albumID)+"/images/"+itoa64(outOfAlbum), token, nil, "")
	resp.Body.Close()

	listing := h.request("GET", "/users/me/albums/"+itoa(albumID)+"/images", token, nil, "")
	defer listing.Body.Close()
	if listing.StatusCode != http.StatusOK {
		t.Fatalf("album images = %d: %s", listing.StatusCode, readAll(t, listing))
	}
	got := imageIDs(t, readAll(t, listing))

	if len(got) != 1 || got[0] != inAlbum {
		t.Errorf("album images = %v, want [%d]", got, inAlbum)
	}
}
