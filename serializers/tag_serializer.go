package serializers

type TagsJsonResponse struct {
	Versatile []interface{} `json:"versatile"`
	Nsfw      []interface{} `json:"nsfw"`
}
