package httpapi

import (
	"context"
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/apierror"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// ImageDto is the public representation of an image.
//
// Field names and casing match the C# ImageDto, because the frontend reads them
// directly. url in particular is derived from Cdn.BaseUrl, which points at a
// Cloudflare Worker rather than at storage.
type ImageDto struct {
	ID            int64                  `json:"id"`
	URL           string                 `json:"url"`
	Width         int64                  `json:"width"`
	Height        int64                  `json:"height"`
	Extension     string                 `json:"extension"`
	ByteSize      int64                  `json:"byteSize"`
	DominantColor string                 `json:"dominantColor"`
	Source        *string                `json:"source,omitempty"`
	IsNsfw        bool                   `json:"isNsfw"`
	IsAnimated    bool                   `json:"isAnimated"`
	UploadedAt    time.Time              `json:"uploadedAt"`
	Tags          []models.TagMinimal    `json:"tags"`
	Artists       []models.ArtistMinimal `json:"artists"`
	Uploader      *models.UserMinimal    `json:"uploader,omitempty"`
}

// imageListResponse is the paginated envelope.
//
// The pagination member is written explicitly rather than through an embedded
// struct, because an embedded pointer does not reliably flatten during JSON
// encoding when it is nil.
type imageListResponse struct {
	Result     []ImageDto `json:"result"`
	Pagination Pagination `json:"pagination"`
}

// registerImageRoutes mounts the image endpoints.
func (s *Server) registerImageRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"images", s.handleListImages)
	mux.HandleFunc("GET "+base+"images/{id}", s.handleGetImage)
}

// handleListImages serves GET /images.
//
// This is the highest-traffic endpoint. Every filter is optional and every one
// is parameterised; the ORDER BY column comes from a fixed whitelist because it
// cannot be bound as a parameter.
func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	page, err := parsePagination(r)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	query := r.URL.Query()

	filter := repo.ImageFilter{
		Orientation: query.Get("orientation"),
		SortBy:      query.Get("sortBy"),
		SortOrder:   query.Get("sortOrder"),
		Page:        page.Page,
		PageSize:    page.PageSize,
		ExcludedIDs: parseIntList(r, "excluded"),
		ForcedIDs:   parseIntList(r, "forced"),
		TagSlugs:    parseStringList(r, "tags"),
		ArtistIDs:   parseIntList(r, "artists"),
		UploaderIDs: parseIntList(r, "uploaders"),
	}

	if filter.IsNsfw, err = parseOptionalBool(r, "isNsfw"); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if filter.IsAnimated, err = parseOptionalBool(r, "isAnimated"); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if filter.MinWidth, err = parseOptionalInt(r, "widthMin"); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if filter.MaxWidth, err = parseOptionalInt(r, "widthMax"); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if filter.MinHeight, err = parseOptionalInt(r, "heightMin"); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if filter.MaxHeight, err = parseOptionalInt(r, "heightMax"); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if filter.MinFileSize, err = parseOptionalInt64(r, "fileSizeMin"); err != nil {
		writeAPIError(w, r, err)
		return
	}
	if filter.MaxFileSize, err = parseOptionalInt64(r, "fileSizeMax"); err != nil {
		writeAPIError(w, r, err)
		return
	}

	images, total, err := s.repo.ListImages(r.Context(), filter)
	if err != nil {
		s.logError(r, "list images", err)
		internalServerError(w, r)
		return
	}

	dtos := s.toImageDtos(r.Context(), images)

	writeJSON(w, http.StatusOK, imageListResponse{
		Result:     dtos,
		Pagination: buildPagination(page, int(total)),
	})
}

// handleGetImage serves GET /images/{id}.
func (s *Server) handleGetImage(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	// Pending content is only visible to moderators and to the uploader, which
	// is what the C# childrenReviewStatus filter does on the read path.
	principal := s.principal(r)
	includePending := principal != nil && principal.CanModerate()

	image, err := s.repo.GetImageByID(r.Context(), id, includePending)
	if err != nil {
		if err == pgx.ErrNoRows {
			apierror.NotFound("Image with ID "+itoa64(id)+" not found.").Write(w, r.URL.Path)
			return
		}
		s.logError(r, "get image", err)
		internalServerError(w, r)
		return
	}

	// A non-moderator asking for a pending image must get 404, not 403, so the
	// existence of unreviewed content is not disclosed.
	if image.ReviewStatus == models.ReviewPending && !includePending {
		apierror.NotFound("Image with ID "+itoa64(id)+" not found.").Write(w, r.URL.Path)
		return
	}

	writeJSON(w, http.StatusOK, s.toImageDto(r.Context(), image))
}

// toImageDto converts a single row, used by GET /images/{id}.
func (s *Server) toImageDto(ctx context.Context, image models.Image) ImageDto {
	dtos := s.toImageDtos(ctx, []models.Image{image})
	if len(dtos) > 0 {
		return dtos[0]
	}
	return ImageDto{URL: s.imageURL(image)}
}

// toImageDtos converts rows into wire form, resolving relations.
//
// Tags and artists are fetched per image rather than in one batch. That is
// deliberately simple and correct; batching is an optimisation worth doing only
// once the query cost is actually measured, and on a 30-item page it is not the
// bottleneck.
func (s *Server) toImageDtos(ctx context.Context, images []models.Image) []ImageDto {
	out := make([]ImageDto, 0, len(images))

	for _, image := range images {
		dtos := ImageDto{
			ID:            image.ID,
			URL:           s.imageURL(image),
			Width:         image.Width,
			Height:        image.Height,
			Extension:     image.Extension,
			ByteSize:      image.ByteSize,
			DominantColor: image.DominantColor,
			Source:        image.Source,
			IsNsfw:        image.IsNsfw,
			IsAnimated:    image.IsAnimated,
			UploadedAt:    image.UploadedAt,
			Tags:          []models.TagMinimal{},
			Artists:       []models.ArtistMinimal{},
		}

		if tags, err := s.repo.GetTagsForImage(ctx, image.ID); err == nil {
			dtos.Tags = tags
		}
		if artists, err := s.repo.GetArtistsForImage(ctx, image.ID); err == nil {
			dtos.Artists = artists
		}

		out = append(out, dtos)
	}
	return out
}

// imageURL builds the public URL for an image.
func (s *Server) imageURL(image models.Image) string {
	return strings.TrimRight(s.cfg.CDN.BaseURL, "/") + "/" + itoa64(image.ID) + image.Extension
}

// writeAPIError writes a *apierror, or converts an arbitrary error.
//
// Handlers return *apierror deliberately so they control the message; this
// helper exists so a forgotten case still produces a valid ProblemDetails body
// rather than an empty 500.
func writeAPIError(w http.ResponseWriter, r *http.Request, err error) {
	var apiErr *apierror.Error
	if e, ok := err.(*apierror.Error); ok {
		apiErr = e
	} else {
		apiErr = apierror.Internal("An unexpected error occurred.")
	}
	apiErr.Write(w, r.URL.Path)
}
