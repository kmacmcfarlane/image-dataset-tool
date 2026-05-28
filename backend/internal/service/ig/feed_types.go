package ig

// feedResponse represents the top-level JSON response from /api/v1/feed/user/{id}/.
type feedResponse struct {
	Items         []feedItem `json:"items"`
	MoreAvailable bool       `json:"more_available"`
	NextMaxID     string     `json:"next_max_id"`
	Status        string     `json:"status"`
}

// feedItem represents a single post in the feed response.
type feedItem struct {
	ID             string         `json:"id"`
	MediaType      int            `json:"media_type"`
	TakenAt        int64          `json:"taken_at"`
	ImageVersions2 imageVersions2 `json:"image_versions2"`
	CarouselMedia  []carouselItem `json:"carousel_media"`
}

// carouselItem represents a single slide in a carousel post.
type carouselItem struct {
	ID             string         `json:"id"`
	MediaType      int            `json:"media_type"`
	ImageVersions2 imageVersions2 `json:"image_versions2"`
}

// imageVersions2 holds the candidate images at various resolutions.
type imageVersions2 struct {
	Candidates []imageCandidate `json:"candidates"`
}

// imageCandidate is a single image resolution variant.
type imageCandidate struct {
	URL    string `json:"url"`
	Width  int    `json:"width"`
	Height int    `json:"height"`
}
