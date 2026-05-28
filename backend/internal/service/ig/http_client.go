package ig

import (
	"context"
	"encoding/json"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"os"
	"path/filepath"
	"strings"
	"time"
)

// Compile-time check that HTTPIGClient implements IGClient.
var _ IGClient = (*HTTPIGClient)(nil)

const (
	igBaseURL         = "https://i.instagram.com"
	feedEndpointFmt   = "/api/v1/feed/user/%s/?count=%d"
	feedEndpointPaged = "/api/v1/feed/user/%s/?count=%d&max_id=%s"
	igReferer         = "https://www.instagram.com/"
	defaultUserAgent  = "Mozilla/5.0 (X11; Linux x86_64; rv:138.0) Gecko/20100101 Firefox/138.0"
	maxErrorDetail    = 500
)

// HTTPIGClient implements IGClient using net/http with browser session cookies.
type HTTPIGClient struct {
	httpClient  *http.Client
	session     *SessionInfo
	logStore    RequestLogStore
	discord     DiscordNotifier
	clock       Clock
	ipResolver  IPResolver
	publicIP    string
	baseURL     string
	logger      *slog.Logger
}

// HTTPIGClientOption configures an HTTPIGClient.
type HTTPIGClientOption func(*HTTPIGClient)

// WithHTTPClient sets a custom http.Client (useful for testing).
func WithHTTPClient(c *http.Client) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.httpClient = c }
}

// WithClock sets a custom clock (useful for testing).
func WithClock(c Clock) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.clock = c }
}

// WithIPResolver sets a custom IP resolver (useful for testing).
func WithIPResolver(r IPResolver) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.ipResolver = r }
}

// WithBaseURL overrides the IG API base URL (for testing with httptest).
func WithBaseURL(url string) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.baseURL = url }
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.logger = l }
}

// NewHTTPIGClient creates a new HTTPIGClient with the given session and dependencies.
func NewHTTPIGClient(
	session *SessionInfo,
	logStore RequestLogStore,
	discord DiscordNotifier,
	opts ...HTTPIGClientOption,
) *HTTPIGClient {
	c := &HTTPIGClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		session:    session,
		logStore:   logStore,
		discord:    discord,
		clock:      RealClock{},
		baseURL:    igBaseURL,
		logger:     slog.With("component", "ig-client"),
	}
	for _, o := range opts {
		o(c)
	}
	return c
}

// ResolveAndCheckIP resolves the public IP and compares it to the last-used IP
// for this account. Should be called once at job start.
func (c *HTTPIGClient) ResolveAndCheckIP() error {
	if c.ipResolver == nil {
		return nil
	}

	ip, err := c.ipResolver.Resolve()
	if err != nil {
		c.logger.Warn("failed to resolve public IP", "error", err)
		return nil // Non-fatal; proceed without IP logging.
	}
	c.publicIP = ip
	c.logger.Info("public IP resolved", "ip", ip)

	lastIP, err := c.logStore.LastIPForAccount(c.session.AccountID)
	if err != nil {
		c.logger.Warn("failed to check last IP", "error", err)
		return nil
	}

	if lastIP != "" && lastIP == ip {
		msg := fmt.Sprintf("Same public IP %s as last session for @%s. Reconnect VPN for IP rotation.", ip, c.session.Handle)
		c.logger.Warn(msg)
		c.discord.Warn(msg)
	}
	return nil
}

// SessionValid checks if the current session has not expired.
func (c *HTTPIGClient) SessionValid() (bool, error) {
	if c.session == nil || c.session.Cookies == "" {
		return false, ErrNoSession
	}
	return c.clock.Now().Before(c.session.ExpiresAt), nil
}

// FetchProfile retrieves basic profile info for a user ID.
// Uses the feed endpoint to extract user info from the first post.
func (c *HTTPIGClient) FetchProfile(ctx context.Context, userID string) (*IGProfile, error) {
	if err := c.checkSession(); err != nil {
		return nil, err
	}

	result, err := c.FetchPosts(ctx, userID, FetchOpts{Count: 1})
	if err != nil {
		return nil, fmt.Errorf("fetch profile via feed: %w", err)
	}

	if len(result.Posts) == 0 {
		return &IGProfile{UserID: userID}, nil
	}

	return &IGProfile{UserID: userID}, nil
}

// FetchPosts fetches posts from a user's feed.
func (c *HTTPIGClient) FetchPosts(ctx context.Context, userID string, opts FetchOpts) (*FeedResult, error) {
	if err := c.checkSession(); err != nil {
		return nil, err
	}

	if opts.Count <= 0 {
		opts.Count = 12
	}

	var endpoint string
	if opts.MaxID != "" {
		endpoint = fmt.Sprintf(feedEndpointPaged, userID, opts.Count, opts.MaxID)
	} else {
		endpoint = fmt.Sprintf(feedEndpointFmt, userID, opts.Count)
	}

	body, status, responseTime, err := c.doRequest(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	c.logRequest(endpoint, status, responseTime, "")

	var feedResp feedResponse
	if err := json.Unmarshal(body, &feedResp); err != nil {
		return nil, fmt.Errorf("parse feed response: %w", err)
	}

	result := &FeedResult{
		MoreAvailable: feedResp.MoreAvailable,
		NextMaxID:     feedResp.NextMaxID,
	}

	for _, item := range feedResp.Items {
		post := c.parsePost(item)
		if post != nil {
			result.Posts = append(result.Posts, *post)
		}
	}

	return result, nil
}

// DownloadMedia downloads a media file from a CDN URL.
// CDN downloads use Referer header only, no session cookies needed.
func (c *HTTPIGClient) DownloadMedia(ctx context.Context, url string, destDir string, filename string) (string, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}

	// CDN downloads only need Referer, not session cookies.
	req.Header.Set("Referer", igReferer)
	req.Header.Set("User-Agent", defaultUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download media: HTTP %d", resp.StatusCode)
	}

	destPath := filepath.Join(destDir, filename)
	f, err := os.Create(destPath)
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	defer f.Close()

	if _, err := io.Copy(f, resp.Body); err != nil {
		return "", fmt.Errorf("write file: %w", err)
	}

	return destPath, nil
}

// checkSession verifies the session is valid before making an API call.
func (c *HTTPIGClient) checkSession() error {
	valid, err := c.SessionValid()
	if err != nil {
		return err
	}
	if !valid {
		msg := fmt.Sprintf("IG session expired for @%s. Re-import cookies to resume.", c.session.Handle)
		c.logger.Error(msg)
		c.discord.Alert(msg)
		return ErrSessionExpired
	}
	return nil
}

// doRequest executes an authenticated request to the IG API.
// Returns body, status code, response time in ms, and error.
// Handles 429/401/403 with logging, Discord alerts, and sentinel errors.
func (c *HTTPIGClient) doRequest(ctx context.Context, endpoint string) ([]byte, int, int64, error) {
	url := c.baseURL + endpoint

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, url, nil)
	if err != nil {
		return nil, 0, 0, fmt.Errorf("create request: %w", err)
	}

	// Set browser-like headers.
	req.Header.Set("User-Agent", defaultUserAgent)
	req.Header.Set("Cookie", c.session.Cookies)
	req.Header.Set("X-CSRFToken", c.session.CSRFToken)
	req.Header.Set("Referer", igReferer)
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	start := c.clock.Now()
	resp, err := c.httpClient.Do(req)
	elapsed := c.clock.Now().Sub(start).Milliseconds()

	if err != nil {
		return nil, 0, elapsed, fmt.Errorf("ig request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		return nil, resp.StatusCode, elapsed, fmt.Errorf("read response: %w", err)
	}

	errDetail := ""
	if resp.StatusCode >= 400 {
		errDetail = truncate(string(body), maxErrorDetail)
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		c.logRequest(endpoint, resp.StatusCode, elapsed, errDetail)
		c.logThrottled(endpoint)
		msg := fmt.Sprintf("IG rate limited (429) on %s for @%s", endpoint, c.session.Handle)
		c.discord.Warn(msg)
		return nil, resp.StatusCode, elapsed, ErrThrottled

	case resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden:
		c.logRequest(endpoint, resp.StatusCode, elapsed, errDetail)
		c.logDenied(endpoint, resp.StatusCode)
		msg := fmt.Sprintf("IG auth failure (%d) on %s for @%s. Job halted.", resp.StatusCode, endpoint, c.session.Handle)
		c.discord.Alert(msg)
		return nil, resp.StatusCode, elapsed, ErrDenied

	case resp.StatusCode >= 400:
		c.logRequest(endpoint, resp.StatusCode, elapsed, errDetail)
		return nil, resp.StatusCode, elapsed, fmt.Errorf("ig API error: HTTP %d", resp.StatusCode)
	}

	return body, resp.StatusCode, elapsed, nil
}

// logRequest writes a request log entry to the store.
func (c *HTTPIGClient) logRequest(endpoint string, status int, responseTimeMs int64, errDetail string) {
	entry := &RequestLog{
		Timestamp:      c.clock.Now(),
		AccountID:      c.session.AccountID,
		Endpoint:       endpoint,
		HTTPStatus:     status,
		IsThrottled:    status == http.StatusTooManyRequests,
		IsDenied:       status == http.StatusUnauthorized || status == http.StatusForbidden,
		ResponseTimeMs: responseTimeMs,
		PublicIP:       c.publicIP,
		ErrorDetail:    errDetail,
	}
	if err := c.logStore.Insert(entry); err != nil {
		c.logger.Error("failed to write ig_request_log", "error", err)
	}
}

// logThrottled logs a throttle event.
func (c *HTTPIGClient) logThrottled(endpoint string) {
	c.logger.Warn("ig request throttled",
		"endpoint", endpoint,
		"account", c.session.Handle,
	)
}

// logDenied logs a denial event.
func (c *HTTPIGClient) logDenied(endpoint string, status int) {
	c.logger.Error("ig request denied",
		"endpoint", endpoint,
		"status", status,
		"account", c.session.Handle,
	)
}

// parsePost converts a raw feed item into an IGPost.
// Returns nil for video-only posts (media_type=2).
func (c *HTTPIGClient) parsePost(item feedItem) *IGPost {
	post := &IGPost{
		ID:        item.ID,
		MediaType: item.MediaType,
		TakenAt:   time.Unix(item.TakenAt, 0).UTC(),
	}

	switch item.MediaType {
	case 1: // Single image.
		img := selectBestImage(item.ImageVersions2.Candidates)
		if img != nil {
			post.Images = []IGImage{
				{URL: img.URL, Width: img.Width, Height: img.Height, SlideIndex: 0},
			}
		}

	case 8: // Carousel.
		for i, cm := range item.CarouselMedia {
			if cm.MediaType == 2 {
				c.logger.Info("skipping video in carousel",
					"post_id", item.ID,
					"slide_index", i,
					"reason", "media_type=2 (video)",
				)
				continue
			}
			img := selectBestImage(cm.ImageVersions2.Candidates)
			if img != nil {
				post.Images = append(post.Images, IGImage{
					URL:        img.URL,
					Width:      img.Width,
					Height:     img.Height,
					SlideIndex: i,
				})
			}
		}

	case 2: // Video — skip.
		c.logger.Info("skipping video post",
			"post_id", item.ID,
			"reason", "media_type=2 (video)",
		)
		return nil

	default:
		c.logger.Warn("unknown media type",
			"post_id", item.ID,
			"media_type", item.MediaType,
		)
		return nil
	}

	return post
}

// selectBestImage picks the candidate with the largest area (width*height).
func selectBestImage(candidates []imageCandidate) *imageCandidate {
	if len(candidates) == 0 {
		return nil
	}
	best := &candidates[0]
	bestArea := best.Width * best.Height
	for i := 1; i < len(candidates); i++ {
		area := candidates[i].Width * candidates[i].Height
		if area > bestArea {
			best = &candidates[i]
			bestArea = area
		}
	}
	return best
}

// truncate limits a string to maxLen characters.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// extractCSRFToken extracts the csrftoken value from a raw cookie string.
func ExtractCSRFToken(cookies string) string {
	for pair := range strings.SplitSeq(cookies, ";") {
		pair = strings.TrimSpace(pair)
		if strings.HasPrefix(pair, "csrftoken=") {
			return strings.TrimPrefix(pair, "csrftoken=")
		}
	}
	return ""
}
