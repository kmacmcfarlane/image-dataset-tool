// Package ig defines the Instagram client interface used by the service layer.
// The HTTP implementation lives in internal/store (store.HTTPIGClient).
package ig

import (
	"context"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// Compile-time check that store.HTTPIGClient implements IGClient.
var _ IGClient = (*store.HTTPIGClient)(nil)

// IGClient defines the interface for interacting with Instagram's API.
type IGClient interface {
	// FetchProfile returns profile metadata for the given user ID.
	FetchProfile(ctx context.Context, userID string) (*model.IGProfile, error)

	// FetchPosts returns posts from a user's feed.
	FetchPosts(ctx context.Context, userID string, opts model.IGFetchOpts) (*model.IGFeedResult, error)

	// DownloadMedia downloads a media file from a CDN URL to the given directory.
	// Returns the local file path of the downloaded file.
	DownloadMedia(ctx context.Context, url string, destDir string, filename string) (string, error)

	// SessionValid checks if the current session is still valid (not expired).
	SessionValid() (bool, error)
}
