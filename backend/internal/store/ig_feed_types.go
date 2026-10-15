package store

// igEnvelope holds the status fields present on every IG API JSON response,
// including error bodies (e.g. {"status":"fail","message":"checkpoint_required"}).
type igEnvelope struct {
	Status  string `json:"status"`
	Message string `json:"message"`
}

// igFeedResponse represents the top-level JSON response from /api/v1/feed/user/{id}/.
type igFeedResponse struct {
	Items         []igFeedItem `json:"items"`
	MoreAvailable bool         `json:"more_available"`
	NextMaxID     string       `json:"next_max_id"`
	Status        string       `json:"status"`
}

// igFeedItem represents a single post in the feed response.
type igFeedItem struct {
	ID             string           `json:"id"`
	MediaType      int              `json:"media_type"`
	TakenAt        int64            `json:"taken_at"`
	User           igFeedUser       `json:"user"`
	ImageVersions2 igImageVersions2 `json:"image_versions2"`
	CarouselMedia  []igCarouselItem `json:"carousel_media"`
}

// igFeedUser is the post owner embedded in each feed item.
type igFeedUser struct {
	Username string `json:"username"`
	FullName string `json:"full_name"`
}

// igCarouselItem represents a single slide in a carousel post.
type igCarouselItem struct {
	ID             string           `json:"id"`
	MediaType      int              `json:"media_type"`
	ImageVersions2 igImageVersions2 `json:"image_versions2"`
}

// igImageVersions2 holds the candidate images at various resolutions.
type igImageVersions2 struct {
	Candidates []igImageCandidate `json:"candidates"`
}

// igImageCandidate is a single image resolution variant.
type igImageCandidate struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
