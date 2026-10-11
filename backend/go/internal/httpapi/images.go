package httpapi

import (
	"net/http"
	"strings"
	"time"

	"github.com/jackc/pgx/v5"

	"waifuapi/internal/apierror"
	"waifuapi/internal/authz"
	"waifuapi/internal/config"
	"waifuapi/internal/repo"
	"waifuapi/internal/store/models"
)

// ImageDto is the wire form of an image.
//
// The field set and order are ImageDto.cs exactly. Several of these are not
// optional decoration: perceptualHash is what the frontend shows on the image
// page, reviewStatus drives the pending badge in the moderation queue,
// uploaderId is what the reports screen uses to attribute a report, and
// favorites/likedAt/albums are what the like button and the album menu render
// from. Omitting any of them breaks a screen.
type ImageDto struct {
	ID             int64          `json:"id"`
	PerceptualHash string         `json:"perceptualHash"`
	Extension      string         `json:"extension"`
	DominantColor  string         `json:"dominantColor"`
	Source         *string        `json:"source"`
	Artists        []ArtistDto    `json:"artists"`
	UploaderID     *int64         `json:"uploaderId"`
	UploadedAt     time.Time      `json:"uploadedAt"`
	IsNsfw         bool           `json:"isNsfw"`
	IsAnimated     bool           `json:"isAnimated"`
	Width          int64          `json:"width"`
	Height         int64          `json:"height"`
	ByteSize       int64          `json:"byteSize"`
	URL            string         `json:"url"`
	Tags           []TagDto       `json:"tags"`
	ReviewStatus   string         `json:"reviewStatus"`
	Favorites      int64          `json:"favorites"`
	LikedAt        *time.Time     `json:"likedAt"`
	AddedToAlbumAt *time.Time     `json:"addedToAlbumAt"`
	Albums         []albumDtoWire `json:"albums"`
}

// albumDtoWire is the embedded album shape on an image.
//
// It carries no description or image count, because the album query on the image
// path projects only what the album menu needs. See GetViewerAlbums.
type albumDtoWire struct {
	ID        int64  `json:"id"`
	Name      string `json:"name"`
	UserID    int64  `json:"userId"`
	IsDefault bool   `json:"isDefault"`
}

// registerImageRoutes mounts the image endpoints.
func (s *Server) registerImageRoutes(mux *http.ServeMux) {
	base := s.cfg.BasePathOrRoot()

	mux.HandleFunc("GET "+base+"images", s.handleListImages)
	mux.HandleFunc("GET "+base+"images/{id}", s.handleGetImage)
}

// handleListImages serves GET /images.
func (s *Server) handleListImages(w http.ResponseWriter, r *http.Request) {
	page, err := parsePagination(r, s.cfg.Pages.Image)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	filter, err := s.imageFilterFromRequest(r, nil)
	if err != nil {
		writeAPIError(w, r, err)
		return
	}
	filter.Page = page.Page
	filter.PageSize = page.PageSize

	s.writeImagePage(w, r, filter, page, 0)
}

// imageFilterFromRequest reads the shared GET /images filter set.
//
// albumID is nil on /images and set on the album endpoint, which is the only
// difference between the two routes' query handling.
//
// The moderator-only parameters are gated here rather than in each handler, so
// no route can accidentally expose them.
func (s *Server) imageFilterFromRequest(r *http.Request, albumID *int64) (repo.ImageFilter, error) {
	query := r.URL.Query()
	var filter repo.ImageFilter
	var err error

	filter.AlbumID = albumID

	if filter.Nsfw, err = models.ParseNsfwMode(query.Get("isNsfw")); err != nil {
		return filter, apierror.BadRequest(err.Error())
	}
	if filter.Animated, err = models.ParseAnimatedMode(query.Get("isAnimated")); err != nil {
		return filter, apierror.BadRequest(err.Error())
	}
	if filter.Orientation, err = models.ParseOrientation(query.Get("orientation")); err != nil {
		return filter, apierror.BadRequest(err.Error())
	}
	if filter.OrderBy, err = models.ParseImageOrderBy(query.Get("orderBy")); err != nil {
		return filter, apierror.BadRequest(err.Error())
	}

	filter.IncludedTags = stringListParam(query, "includedTags")
	filter.ExcludedTags = stringListParam(query, "excludedTags")
	filter.IncludedArtists = stringListParam(query, "includedArtists")
	filter.ExcludedArtists = stringListParam(query, "excludedArtists")
	filter.IncludedIDs = stringListParam(query, "includedIds")
	filter.ExcludedIDs = stringListParam(query, "excludedIds")

	filter.Width = query.Get("width")
	filter.Height = query.Get("height")
	filter.ByteSize = query.Get("byteSize")

	// The caller identity is taken from the token, never from the query string.
	// The album endpoint additionally knows which user the album belongs to, and
	// passes it so a moderator browsing someone else's album still gets the
	// per-viewer liked/albums fields for themselves.
	caller := s.principal(r)
	if caller != nil {
		id := caller.UserID
		filter.UserID = &id
	}
	if viewerID := s.viewerIDForAlbum(r, albumID); viewerID != 0 {
		id := viewerID
		filter.UserID = &id
	}

	filter.IncludeMyPending = truthy(query.Get("includeMyPending"))
	filter.ChildrenReviewStatus = models.FilterAccepted
	if raw := query.Get("childrenReviewStatus"); raw != "" {
		if filter.ChildrenReviewStatus, err = models.ParseReviewStatusFilter(raw); err != nil {
			return filter, apierror.BadRequest(err.Error())
		}
	}
	filter.IncludeMyPendingChildren = truthy(query.Get("includeMyPendingChildren"))

	// Filtering by uploader or by a non-accepted review status exposes
	// unreviewed content, so it is moderator-only. A non-moderator asking for it
	// gets 403 rather than silently having the filter dropped.
	if raw := query.Get("uploaderId"); raw != "" {
		if !s.canModerate(r) {
			return filter, &authz.ErrForbidden{
				Action:       authz.Action("filter images by uploader"),
				RequiredRole: config.RoleModerator,
			}
		}
		id, parseErr := parsePositiveInt64(raw)
		if parseErr != nil {
			return filter, apierror.BadRequest("'uploaderId' must be a number.")
		}
		filter.UploaderID = &id
	}

	if raw := query.Get("reviewStatus"); raw != "" {
		if filter.ReviewStatus, err = models.ParseReviewStatusFilter(raw); err != nil {
			return filter, apierror.BadRequest(err.Error())
		}
		if filter.ReviewStatus != models.FilterAccepted && !s.canModerate(r) {
			return filter, &authz.ErrForbidden{
				Action:       authz.Action("filter images by review status"),
				RequiredRole: config.RoleModerator,
			}
		}
	}

	if filter.ChildrenReviewStatus != models.FilterAccepted && !s.canModerate(r) {
		return filter, &authz.ErrForbidden{
			Action:       authz.Action("filter image tags and artists by review status"),
			RequiredRole: config.RoleModerator,
		}
	}

	// AddedToAlbum is meaningless without an album to sort within.
	if filter.OrderBy == models.OrderAddedToAlbum && albumID == nil {
		return filter, apierror.BadRequest(
			"AddedToAlbum ordering is only available when viewing an album.").
			WithExtra("errors", map[string][]string{
				"orderBy": {"AddedToAlbum ordering is only available when viewing an album."},
			})
	}

	return filter, nil
}

// writeImagePage runs the query and renders the paginated response.
//
// albumID is non-zero on the album endpoint and supplies the addedToAlbumAt
// field.
func (s *Server) writeImagePage(w http.ResponseWriter, r *http.Request, filter repo.ImageFilter, page pageParams, albumID int64) {
	images, total, err := s.repo.ListImages(r.Context(), filter)
	if err != nil {
		s.logError(r, "list images", err)
		internalServerError(w, r)
		return
	}

	dtos, err := s.toImageDtos(r, images, filter, albumID)
	if err != nil {
		s.logError(r, "resolve image relations", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, buildPaginatedList(page, dtos, int(total)))
}

// handleGetImage serves GET /images/{id}.
//
// The two query parameters are not decoration. ImagePage.tsx sends
// childrenReviewStatus=All when a moderator opens a pending image and
// includeMyPendingChildren=true when an ordinary user views their own upload,
// and both are load-bearing: without them the page renders with empty tag and
// artist lists exactly when somebody is trying to see why a submission looks
// wrong.
func (s *Server) handleGetImage(w http.ResponseWriter, r *http.Request) {
	id, err := parsePathInt64(r, "id")
	if err != nil {
		writeAPIError(w, r, err)
		return
	}

	query := r.URL.Query()

	childrenStatus := models.FilterAccepted
	if raw := query.Get("childrenReviewStatus"); raw != "" {
		if childrenStatus, err = models.ParseReviewStatusFilter(raw); err != nil {
			writeAPIError(w, r, validationError("ChildrenReviewStatus", err.Error()))
			return
		}
		// Seeing unreviewed tags and artists is moderator-only here, exactly as
		// on the gallery endpoint.
		if childrenStatus != models.FilterAccepted && !s.canModerate(r) {
			writeAPIError(w, r, &authz.ErrForbidden{
				Action:       authz.Action("filter image tags and artists by review status"),
				RequiredRole: config.RoleModerator,
			})
			return
		}
	}
	includeMyPendingChildren := truthy(query.Get("includeMyPendingChildren"))

	// Pending content is visible to moderators and to the uploader, which is what
	// the C# childrenReviewStatus filter does on the read path.
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

	filter := repo.ImageFilter{
		ChildrenReviewStatus:     childrenStatus,
		ReviewStatus:             models.FilterAll,
		IncludeMyPendingChildren: includeMyPendingChildren,
	}
	if principal != nil {
		viewer := principal.UserID
		filter.UserID = &viewer
	}

	dtos, err := s.toImageDtos(r, []models.Image{image}, filter, 0)
	if err != nil || len(dtos) == 0 {
		s.logError(r, "resolve image relations", err)
		internalServerError(w, r)
		return
	}

	writeJSON(w, http.StatusOK, dtos[0])
}

// toImageDtos converts a page of rows into wire form, resolving relations in
// one batch per relation rather than per image.
//
// A default page is 30 images; resolving tags, artists, favorites, likes and
// albums per image would be 150 round trips, which on a 0.1-CPU instance is the
// difference between a responsive gallery and a timeout.
func (s *Server) toImageDtos(r *http.Request, images []models.Image, filter repo.ImageFilter, albumID int64) ([]ImageDto, error) {
	out := make([]ImageDto, 0, len(images))
	if len(images) == 0 {
		return out, nil
	}
	ctx := r.Context()

	ids := make([]int64, 0, len(images))
	for _, image := range images {
		ids = append(ids, image.ID)
	}

	viewerID := int64(0)
	if filter.UserID != nil {
		viewerID = *filter.UserID
	}

	visibility := repo.ChildVisibility{
		Filter:      filter.ChildrenReviewStatus,
		ViewerID:    viewerID,
		IncludeMine: filter.IncludeMyPendingChildren,
	}

	tagsByImage, err := s.repo.GetTagsForImages(ctx, ids, visibility)
	if err != nil {
		return nil, err
	}
	artistsByImage, err := s.repo.GetArtistsForImages(ctx, ids, visibility)
	if err != nil {
		return nil, err
	}
	favorites, err := s.repo.GetFavorites(ctx, ids)
	if err != nil {
		return nil, err
	}
	likedAt, err := s.repo.GetViewerLikes(ctx, ids, viewerID)
	if err != nil {
		return nil, err
	}
	viewerAlbums, err := s.repo.GetViewerAlbums(ctx, ids, viewerID)
	if err != nil {
		return nil, err
	}
	addedToAlbum, err := s.repo.GetAddedToAlbumAt(ctx, ids, albumID)
	if err != nil {
		return nil, err
	}

	// Uploader ids are only exposed to moderators, matching the includeUploaderId
	// flag on ToDto. Doing it here rather than per image keeps it to one check.
	includeUploader := s.canModerate(r)

	for _, image := range images {
		dto := ImageDto{
			ID:             image.ID,
			PerceptualHash: models.PerceptualHashToHex(image.PerceptualHash),
			Extension:      image.Extension,
			DominantColor:  image.DominantColor,
			Source:         image.Source,
			UploadedAt:     image.UploadedAt,
			IsNsfw:         image.IsNsfw,
			IsAnimated:     image.IsAnimated,
			Width:          image.Width,
			Height:         image.Height,
			ByteSize:       image.ByteSize,
			URL:            s.imageURL(image),
			ReviewStatus:   image.ReviewStatus.String(),
			Favorites:      favorites[image.ID],
			Tags:           []TagDto{},
			Artists:        []ArtistDto{},
			Albums:         []albumDtoWire{},
		}

		if includeUploader {
			dto.UploaderID = image.UploaderID
		}

		for _, tag := range tagsByImage[image.ID] {
			dto.Tags = append(dto.Tags, TagDto{
				ID: tag.ID, Name: tag.Name, Slug: tag.Slug,
				Description:  tag.Description,
				ReviewStatus: strPtr(tag.ReviewStatus.String()),
				CreatorID:    tag.CreatorID,
			})
		}
		for _, artist := range artistsByImage[image.ID] {
			dto.Artists = append(dto.Artists, toArtistDto(artist))
		}
		for _, album := range viewerAlbums[image.ID] {
			dto.Albums = append(dto.Albums, albumDtoWire{
				ID: album.ID, Name: album.Name, UserID: album.UserID, IsDefault: album.IsDefault,
			})
		}

		if at, ok := likedAt[image.ID]; ok {
			value := at
			dto.LikedAt = &value
		}
		if at, ok := addedToAlbum[image.ID]; ok {
			value := at
			dto.AddedToAlbumAt = &value
		}

		out = append(out, dto)
	}

	return out, nil
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
	switch e := err.(type) {
	case *apierror.Error:
		apiErr = e
	case *authz.ErrForbidden:
		apiErr = apierror.Forbidden(e.Error())
	default:
		apiErr = apierror.Internal("An unexpected error occurred.")
	}
	apiErr.Write(w, r.URL.Path)
}
