package model

import "time"

// IGRequestLog is a record of an IG API request for observability.
type IGRequestLog struct {
	ID             int64
	Timestamp      time.Time
	AccountID      string
	Endpoint       string
	HTTPStatus     int
	IsThrottled    bool
	IsDenied       bool
	ResponseTimeMs int64
	PublicIP       string
	ErrorDetail    string
}

// IGSession holds the session state for an IG account.
type IGSession struct {
	AccountID string
	Handle    string
	Cookies   string // Raw cookie header string.
	CSRFToken string // Extracted from cookies for x-csrftoken header.
	ExpiresAt time.Time
}

// IGFetchOpts configures a FetchPosts call.
type IGFetchOpts struct {
	Count int    // Number of posts to request per page.
	MaxID string // Pagination cursor for next page.
}

// IGProfile represents basic IG user profile metadata.
type IGProfile struct {
	UserID   string
	Username string
	FullName string
}

// IGFeedResult holds the parsed response from the feed endpoint.
type IGFeedResult struct {
	Posts         []IGPost
	MoreAvailable bool
	NextMaxID     string
}

// IGPost represents a single post from the feed.
type IGPost struct {
	ID        string
	MediaType int // 1=image, 2=video, 8=carousel
	TakenAt   time.Time
	// Images holds the best-resolution image URL(s) for this post.
	// For single images (media_type=1), this has one entry.
	// For carousels (media_type=8), this has one entry per image slide.
	Images []IGImage
}

// IGImage represents a single downloadable image with metadata.
type IGImage struct {
	URL        string
	Width      int
	Height     int
	SlideIndex int // 0 for single-image posts; 0..N for carousel slides.
}
