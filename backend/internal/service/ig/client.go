// Package ig implements the Instagram client interface and HTTP implementation
// for fetching posts from the Instagram REST feed API using browser session cookies.
package ig

import (
	"context"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

// RequestLog is an alias for model.IGRequestLog for convenience.
type RequestLog = model.IGRequestLog

// IGClient defines the interface for interacting with Instagram's API.
type IGClient interface {
	// FetchProfile returns profile metadata for the given user ID.
	FetchProfile(ctx context.Context, userID string) (*IGProfile, error)

	// FetchPosts returns posts from a user's feed.
	FetchPosts(ctx context.Context, userID string, opts FetchOpts) (*FeedResult, error)

	// DownloadMedia downloads a media file from a CDN URL to the given directory.
	// Returns the local file path of the downloaded file.
	DownloadMedia(ctx context.Context, url string, destDir string, filename string) (string, error)

	// SessionValid checks if the current session is still valid (not expired).
	SessionValid() (bool, error)
}

// FetchOpts configures a FetchPosts call.
type FetchOpts struct {
	Count  int    // Number of posts to request per page.
	MaxID  string // Pagination cursor for next page.
}

// IGProfile represents basic IG user profile metadata.
type IGProfile struct {
	UserID   string
	Username string
	FullName string
}

// FeedResult holds the parsed response from the feed endpoint.
type FeedResult struct {
	Posts         []IGPost
	MoreAvailable bool
	NextMaxID     string
}

// IGPost represents a single post from the feed.
type IGPost struct {
	ID        string
	MediaType int       // 1=image, 2=video, 8=carousel
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

