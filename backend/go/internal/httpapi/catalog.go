package httpapi

import (
	"net/http"
	"net/url"
	"strings"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/config"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// TagDto is the wire form of a tag.
//
// It is TagDto.cs. ReviewStatus and CreatorId are nullable and imageCount is
// present because the filter sidebar renders a tag's usage count and the
// metadata editor shows whether a tag is still pending.
type TagDto struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Slug         string  `json:"slug"`
	Description  string  `json:"description"`
	ReviewStatus *string `json:"reviewStatus"`
	CreatorID    *int64  `json:"creatorId"`
	ImageCount   int64   `json:"imageCount"`
}

// ArtistDto is the wire form of an artist.
//
// ArtistDto.ReviewStatus is not nullable in C#, so it is a plain string here.
type ArtistDto struct {
	ID           int64   `json:"id"`
	Name         string  `json:"name"`
	Patreon      *string `json:"patreon"`
	Pixiv        *string `json:"pixiv"`
	Twitter      *string `json:"twitter"`
	DeviantArt   *string `json:"deviantArt"`
	ReviewStatus string  `json:"reviewStatus"`
	CreatorID    *int64  `json:"creatorId"`
	ImageCount   int64   `json:"imageCount"`
}

// toTagDto converts a catalogue entry.
//
// creatorID is passed through only when exposeCreator is true, which the
// repository has already decided; passing nil here means "hidden".
func toTagDto(entry repo.TagEntry, exposeCreator bool) TagDto {
	status := entry.ReviewStatus.String()
	dto := TagDto{
		ID:           entry.ID,
		Name:         entry.Name,
		Slug:         entry.Slug,
		Description:  entry.Description,
		ReviewStatus: &status,
		ImageCount:   entry.ImageCount,
	}
	if exposeCreator {
		dto.CreatorID = entry.CreatorID
	}
	return dto
}

// toArtistDto converts an artist row.
func toArtistDto(artist models.Artist) ArtistDto {
	return ArtistDto{
		ID:           artist.ID,
		Name:         artist.Name,
		Patreon:      artist.Patreon,
		Pixiv:        artist.Pixiv,
		Twitter:      artist.Twitter,
		DeviantArt:   artist.DeviantArt,
		ReviewStatus: artist.ReviewStatus.String(),
		CreatorID:    artist.CreatorID,
	}
}

func toArtistEntryDto(entry repo.ArtistEntry, exposeCreator bool) ArtistDto {
	dto := toArtistDto(entry.Artist)
	dto.ImageCount = entry.ImageCount
	if !exposeCreator {
		dto.CreatorID = nil
	}
	return dto
}

// registerCatalogRoutes mounts the tag and artist read endpoints.
//
// The mutations are registered separately, in catalog_write.go, because their
// permission rules differ per verb.
func (s *Server) registerCatalogRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"tags", s.handleListTags)
	mux.HandleFunc("GET "+base+"tags/by-slug/{slug}", s.handleGetTagBySlug)
	mux.HandleFunc("GET "+base+"tags/{id}", s.handleGetTag)

	mux.HandleFunc("GET "+base+"artists", s.handleListArtists)
	mux.HandleFunc("GET "+base+"artists/by-name/{name}", s.handleGetArtistByName)
	mux.HandleFunc("GET "+base+"artists/{id}", s.handleGetArtist)
}

// catalogFilterFromRequest reads the shared GET /tags and GET /artists filters.
//
// The review-status filter is moderator-only: an ordinary caller asking for
// pending or all content is refused with 403 rather than silently given the
// accepted-only list, because the C# service refuses rather than downgrading.
func (s *Server) catalogFilterFromRequest(r *http.Request) (repo.CatalogFilter, error) {
	query := r.URL.Query()
	var filter repo.CatalogFilter

	filter.Name = query.Get("name")
	filter.IncludedIDs = parseIntList(r, "includedIds")
	if slugs := stringListParam(query, "includedSlugs"); len(slugs) > 0 {
		filter.IncludedSlugs = slugs
	}
	filter.IncludeMine = truthy(query.Get("includeMyPending"))

	if caller := s.principal(r); caller != nil {
		filter.ViewerID = caller.UserID
	}

	filter.ReviewStatus = models.FilterAccepted
	if raw := query.Get("reviewStatus"); raw != "" {
		status, err := models.ParseReviewStatusFilter(raw)
		if err != nil {
			return filter, apierror.BadRequest(err.Error())
		}
		filter.ReviewStatus = status
	}

	if filter.ReviewStatus != models.FilterAccepted && !s.canModerate(r) {
		return filter, &authz.ErrForbidden{
			Action:       authz.Action("filter by review status"),
			RequiredRole: config.RoleModerator,
		}
	}

	return filter, nil
}

// handleListTags serves GET /tags.
func (s *Server) handleListTags(w http.ResponseWriter, r *http.Request) {
	page, err := parsePagination(r, s.cfg.Pages.Tag)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	filter, err := s.catalogFilterFromRequest(r)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	filter.Page = page.Page
	filter.PageSize = page.PageSize

	exposeCreator := s.canModerate(r)
	entries, total, err := s.repo.ListTags(r.Context(), filter, exposeCreator)
	if err != nil {
		s.logError(r, "list tags", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]TagDto, 0, len(entries))
	for _, entry := range entries {
		dtos = append(dtos, toTagDto(entry, exposeCreator))
	}

	// An exact-lookup request is not paginated in the C# service: it returns one
	// page holding everything that matched and reports pageNumber 1.
	if exactLookup(filter) {
		page = exactLookupPage(page, len(dtos))
		total = int64(len(dtos))
	}

	writeJSON(w, http.StatusOK, buildPaginatedList(page, dtos, int(total)))
}

// exactLookup reports whether the request asked for specific tags or artists.
func exactLookup(filter repo.CatalogFilter) bool {
	return len(filter.IncludedIDs) > 0 || len(filter.IncludedSlugs) > 0
}

// exactLookupPage reshapes the envelope for an unpaginated lookup.
func exactLookupPage(page pageParams, count int) pageParams {
	size := count
	if size == 0 {
		size = 1
	}
	page.Page = 0
	page.PageSize = size
	return page
}

// handleGetTag serves GET /tags/{id}.
func (s *Server) handleGetTag(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	tag, err := s.repo.GetTagByID(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get tag", err, "Tag with ID "+itoa64(id)+" not found.")
		return
	}

	// ReviewStatus is carried, matching GetTagById/Query.cs. CreatorId and
	// ImageCount are deliberately left unset: that projection does not populate
	// them either, so a single-tag lookup reports null and 0 there too, and
	// filling them here would be a divergence in the other direction.
	status := tag.ReviewStatus.String()
	writeJSON(w, http.StatusOK, TagDto{
		ID:           tag.ID,
		Name:         tag.Name,
		Slug:         tag.Slug,
		Description:  tag.Description,
		ReviewStatus: &status,
	})
}

// handleGetTagBySlug serves GET /tags/by-slug/{slug}.
func (s *Server) handleGetTagBySlug(w http.ResponseWriter, r *http.Request) {
	slug, err := url.PathUnescape(r.PathValue("slug"))
	if err != nil {
		writeAPIError(w, r, apierror.BadRequest("'slug' is not valid."))
		return
	}

	tag, err := s.repo.GetTagBySlug(r.Context(), slug)
	if err != nil {
		writeRepoError(w, r, "get tag by slug", err, "Tag with slug '"+slug+"' not found.")
		return
	}

	status := tag.ReviewStatus.String()
	writeJSON(w, http.StatusOK, TagDto{
		ID:           tag.ID,
		Name:         tag.Name,
		Slug:         tag.Slug,
		Description:  tag.Description,
		ReviewStatus: &status,
	})
}

// handleListArtists serves GET /artists.
func (s *Server) handleListArtists(w http.ResponseWriter, r *http.Request) {
	page, err := parsePagination(r, s.cfg.Pages.Artist)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	filter, err := s.catalogFilterFromRequest(r)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	filter.Page = page.Page
	filter.PageSize = page.PageSize

	exposeCreator := s.canModerate(r)
	entries, total, err := s.repo.ListArtists(r.Context(), filter, exposeCreator)
	if err != nil {
		s.logError(r, "list artists", err)
		internalServerError(w, r)
		return
	}

	dtos := make([]ArtistDto, 0, len(entries))
	for _, entry := range entries {
		dtos = append(dtos, toArtistEntryDto(entry, exposeCreator))
	}

	if exactLookup(filter) {
		page = exactLookupPage(page, len(dtos))
		total = int64(len(dtos))
	}

	writeJSON(w, http.StatusOK, buildPaginatedList(page, dtos, int(total)))
}

// handleGetArtist serves GET /artists/{id}.
func (s *Server) handleGetArtist(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	artist, err := s.repo.GetArtistByID(r.Context(), id)
	if err != nil {
		writeRepoError(w, r, "get artist", err, "Artist with ID "+itoa64(id)+" not found.")
		return
	}

	writeJSON(w, http.StatusOK, toArtistDto(artist))
}

// handleGetArtistByName serves GET /artists/by-name/{name}.
func (s *Server) handleGetArtistByName(w http.ResponseWriter, r *http.Request) {
	name, err := url.PathUnescape(r.PathValue("name"))
	if err != nil {
		writeAPIError(w, r, apierror.BadRequest("'name' is not valid."))
		return
	}

	artist, err := s.repo.GetArtistByName(r.Context(), strings.TrimSpace(name))
	if err != nil {
		writeRepoError(w, r, "get artist by name", err,
			"Artist with name '"+name+"' not found.")
		return
	}

	writeJSON(w, http.StatusOK, toArtistDto(artist))
}
