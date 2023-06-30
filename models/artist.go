package models

// Artist Represent an artist to a json-like format
type Artist struct {
	ArtistId   int64   `field:"artist_id" json:"artist_id" example:"1"`
	Name       string  `field:"name" json:"name" example:"fourthwallzart"`
	Patreon    *string `field:"patreon" json:"patreon"`
	Pixiv      *string `field:"pixiv" json:"pixiv"`
	Twitter    *string `field:"twitter" json:"twitter" example:"https://twitter.com/4thWallzArt"`
	DeviantArt *string `field:"deviant_art" json:"deviant_art" example:"https://www.deviantart.com/4thwallzart"`
}
