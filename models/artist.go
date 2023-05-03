package models

// Artist Represent an artist to a json-like format
type Artist struct {
	ArtistId   int64   `field:"artist_id" json:"artist_id"`
	Name       string  `field:"name" json:"name"`
	Patreon    *string `field:"patreon" json:"patreon"`
	Pixiv      *string `field:"pixiv" json:"pixiv"`
	Twitter    *string `field:"twitter" json:"twitter"`
	DeviantArt *string `field:"deviant_art" json:"deviant_art"`
}
