package constants

// The image package use some of those values as default and bind the query params corresponding to them
// the database package then use the passed query params to generate the query dynamically
// they are located in database package to avoid any circular import
const (
	Null  = "null"
	True  = "true"
	False = "false"
)

const (
	Favorites  = "FAVORITES"
	UploadedAt = "UPLOADED_AT"
	LikedAt    = "LIKED_AT"
	Random     = "RANDOM"
)

const (
	Landscape = "LANDSCAPE"
	Portrait  = "PORTRAIT"
)

const ManyLimit = "30"
