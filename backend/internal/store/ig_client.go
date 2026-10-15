package store

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"log/slog"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"strconv"
	"strings"
	"time"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
)

const (
	igBaseURL        = "https://i.instagram.com"
	igFeedPathFmt    = "/api/v1/feed/user/%s/"
	igReferer        = "https://www.instagram.com/"
	igUserAgent      = "Mozilla/5.0 (X11; Linux x86_64; rv:138.0) Gecko/20100101 Firefox/138.0"
	igDefaultCount   = 12
	igMaxErrorDetail = 500
)

// igDenialMessages are the IG API "message" values that mean the session is no
// longer usable and the job must halt (in addition to HTTP 401/403).
var igDenialMessages = map[string]bool{
	"checkpoint_required": true,
	"challenge_required":  true,
	"login_required":      true,
}

// IGBackoff configures how HTTPIGClient retries a request that returned 429.
// Delays grow exponentially from Initial, capped at Max.
type IGBackoff struct {
	MaxRetries int
	Initial    time.Duration
	Max        time.Duration
}

// DefaultIGBackoff is the backoff policy used unless WithBackoff overrides it.
var DefaultIGBackoff = IGBackoff{MaxRetries: 3, Initial: 30 * time.Second, Max: 5 * time.Minute}

// delay returns the wait before retry number attempt (0-based).
func (b IGBackoff) delay(attempt int) time.Duration {
	d := b.Initial
	for i := 0; i < attempt && d < b.Max; i++ {
		d *= 2
	}
	if d > b.Max {
		d = b.Max
	}
	return d
}

// HTTPIGClient implements the service-layer IGClient interface using net/http
// with browser session cookies.
type HTTPIGClient struct {
	httpClient *http.Client
	session    *model.IGSession
	logStore   IGRequestLogWriter
	discord    DiscordNotifier
	clock      Clock
	sleeper    Sleeper
	backoff    IGBackoff
	ipResolver IPResolver
	publicIP   string
	baseURL    string
	logger     *slog.Logger
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

// WithSleeper sets a custom sleeper used for 429 backoff (useful for testing).
func WithSleeper(s Sleeper) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.sleeper = s }
}

// WithBackoff sets the 429 backoff policy.
func WithBackoff(b IGBackoff) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.backoff = b }
}

// WithIPResolver sets a custom IP resolver (useful for testing).
func WithIPResolver(r IPResolver) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.ipResolver = r }
}

// WithBaseURL overrides the IG API base URL (for testing with httptest).
func WithBaseURL(u string) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.baseURL = u }
}

// WithLogger sets a custom logger.
func WithLogger(l *slog.Logger) HTTPIGClientOption {
	return func(h *HTTPIGClient) { h.logger = l }
}

// NewHTTPIGClient creates a new HTTPIGClient with the given session and dependencies.
func NewHTTPIGClient(
	session *model.IGSession,
	logStore IGRequestLogWriter,
	discord DiscordNotifier,
	opts ...HTTPIGClientOption,
) *HTTPIGClient {
	c := &HTTPIGClient{
		httpClient: &http.Client{Timeout: 30 * time.Second},
		session:    session,
		logStore:   logStore,
		discord:    discord,
		clock:      RealClock{},
		sleeper:    RealSleeper{},
		backoff:    DefaultIGBackoff,
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
		return false, ErrIGNoSession
	}
	return c.clock.Now().Before(c.session.ExpiresAt), nil
}

// FetchProfile retrieves basic profile info for a user ID. It costs one feed
// request (count=1) and reads the owner's username and full name from the
// first post; both stay empty when the user has no posts.
func (c *HTTPIGClient) FetchProfile(ctx context.Context, userID string) (*model.IGProfile, error) {
	feed, err := c.fetchFeed(ctx, userID, model.IGFetchOpts{Count: 1})
	if err != nil {
		return nil, fmt.Errorf("fetch profile via feed: %w", err)
	}

	profile := &model.IGProfile{UserID: userID}
	if len(feed.Items) > 0 {
		profile.Username = feed.Items[0].User.Username
		profile.FullName = feed.Items[0].User.FullName
	}
	return profile, nil
}

// FetchPosts fetches posts from a user's feed.
func (c *HTTPIGClient) FetchPosts(ctx context.Context, userID string, opts model.IGFetchOpts) (*model.IGFeedResult, error) {
	feed, err := c.fetchFeed(ctx, userID, opts)
	if err != nil {
		return nil, err
	}

	result := &model.IGFeedResult{
		MoreAvailable: feed.MoreAvailable,
		NextMaxID:     feed.NextMaxID,
	}
	for _, item := range feed.Items {
		if post := c.parsePost(item); post != nil {
			result.Posts = append(result.Posts, *post)
		}
	}
	return result, nil
}

// DownloadMedia downloads a media file from a CDN URL into destDir/filename.
// CDN downloads use Referer header only, no session cookies needed.
// filename must be a plain file name; the file appears only once fully written.
func (c *HTTPIGClient) DownloadMedia(ctx context.Context, mediaURL string, destDir string, filename string) (string, error) {
	if filename == "" || filename == "." || filename == ".." || strings.ContainsAny(filename, `/\`) {
		return "", fmt.Errorf("invalid filename %q: must be a plain file name", filename)
	}

	req, err := http.NewRequestWithContext(ctx, http.MethodGet, mediaURL, nil)
	if err != nil {
		return "", fmt.Errorf("create download request: %w", err)
	}

	// CDN downloads only need Referer, not session cookies.
	req.Header.Set("Referer", igReferer)
	req.Header.Set("User-Agent", igUserAgent)

	resp, err := c.httpClient.Do(req)
	if err != nil {
		return "", fmt.Errorf("download media: %w", err)
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		return "", fmt.Errorf("download media: HTTP %d", resp.StatusCode)
	}

	destPath := filepath.Join(destDir, filename)
	tmp, err := os.CreateTemp(destDir, "."+filename+".part-*")
	if err != nil {
		return "", fmt.Errorf("create file: %w", err)
	}
	tmpPath := tmp.Name()

	if _, err := io.Copy(tmp, resp.Body); err != nil {
		tmp.Close()
		os.Remove(tmpPath)
		return "", fmt.Errorf("write file: %w", err)
	}
	if err := tmp.Close(); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("close file: %w", err)
	}
	if err := os.Rename(tmpPath, destPath); err != nil {
		os.Remove(tmpPath)
		return "", fmt.Errorf("rename file: %w", err)
	}

	c.logger.Info("media downloaded", "path", destPath)
	return destPath, nil
}

// fetchFeed requests one page of a user's feed and decodes it.
func (c *HTTPIGClient) fetchFeed(ctx context.Context, userID string, opts model.IGFetchOpts) (*igFeedResponse, error) {
	if opts.Count <= 0 {
		opts.Count = igDefaultCount
	}
	q := url.Values{}
	q.Set("count", strconv.Itoa(opts.Count))
	if opts.MaxID != "" {
		q.Set("max_id", opts.MaxID)
	}
	endpoint := fmt.Sprintf(igFeedPathFmt, url.PathEscape(userID)) + "?" + q.Encode()

	body, err := c.getWithBackoff(ctx, endpoint)
	if err != nil {
		return nil, err
	}

	var feed igFeedResponse
	if err := json.Unmarshal(body, &feed); err != nil {
		return nil, fmt.Errorf("parse feed response: %w", err)
	}
	return &feed, nil
}

// getWithBackoff performs an authenticated GET, retrying 429 responses with
// exponential backoff. The session is checked before every attempt. A Discord
// warning is sent on the first 429; ErrIGThrottled is returned once retries
// are exhausted.
func (c *HTTPIGClient) getWithBackoff(ctx context.Context, endpoint string) ([]byte, error) {
	for attempt := 0; ; attempt++ {
		if err := c.checkSession(); err != nil {
			return nil, err
		}

		body, err := c.doRequest(ctx, endpoint)
		if !errors.Is(err, ErrIGThrottled) {
			return body, err
		}

		if attempt == 0 {
			c.discord.Warn(fmt.Sprintf("IG rate limited (429) on %s for @%s. Backing off.", endpoint, c.session.Handle))
		}
		if attempt >= c.backoff.MaxRetries {
			return nil, err
		}

		d := c.backoff.delay(attempt)
		c.logger.Warn("ig request throttled, backing off",
			"endpoint", endpoint,
			"account", c.session.Handle,
			"retry", attempt+1,
			"delay", d,
		)
		if err := c.sleeper.Sleep(ctx, d); err != nil {
			return nil, fmt.Errorf("ig backoff interrupted: %w", err)
		}
	}
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
		return ErrIGSessionExpired
	}
	return nil
}

// doRequest executes one authenticated request to the IG API and writes one
// ig_request_log row for it, whatever the outcome. 429 returns ErrIGThrottled;
// 401/403 or a checkpoint/challenge/login message returns ErrIGDenied and
// sends a Discord alert.
func (c *HTTPIGClient) doRequest(ctx context.Context, endpoint string) ([]byte, error) {
	req, err := http.NewRequestWithContext(ctx, http.MethodGet, c.baseURL+endpoint, nil)
	if err != nil {
		return nil, fmt.Errorf("create request: %w", err)
	}

	// Set browser-like headers.
	req.Header.Set("User-Agent", igUserAgent)
	req.Header.Set("Cookie", c.session.Cookies)
	req.Header.Set("X-CSRFToken", c.session.CSRFToken)
	req.Header.Set("Referer", igReferer)
	req.Header.Set("X-IG-App-ID", "936619743392459")
	req.Header.Set("X-Requested-With", "XMLHttpRequest")

	start := c.clock.Now()
	resp, err := c.httpClient.Do(req)
	elapsed := c.clock.Now().Sub(start).Milliseconds()

	if err != nil {
		c.logRequest(endpoint, 0, elapsed, false, false, truncate(err.Error(), igMaxErrorDetail))
		return nil, fmt.Errorf("ig request failed: %w", err)
	}
	defer resp.Body.Close()

	body, err := io.ReadAll(resp.Body)
	if err != nil {
		c.logRequest(endpoint, resp.StatusCode, elapsed, false, false, truncate("read response: "+err.Error(), igMaxErrorDetail))
		return nil, fmt.Errorf("read response: %w", err)
	}

	var env igEnvelope
	_ = json.Unmarshal(body, &env) // Best effort: non-JSON bodies leave env empty.
	statusFailed := env.Status != "" && env.Status != "ok"

	errDetail := ""
	if resp.StatusCode >= 400 || statusFailed {
		errDetail = truncate(string(body), igMaxErrorDetail)
	}

	switch {
	case resp.StatusCode == http.StatusTooManyRequests:
		c.logRequest(endpoint, resp.StatusCode, elapsed, true, false, errDetail)
		return nil, ErrIGThrottled

	case resp.StatusCode == http.StatusUnauthorized ||
		resp.StatusCode == http.StatusForbidden ||
		igDenialMessages[env.Message]:
		c.logRequest(endpoint, resp.StatusCode, elapsed, false, true, errDetail)
		c.logger.Error("ig request denied",
			"endpoint", endpoint,
			"status", resp.StatusCode,
			"message", env.Message,
			"account", c.session.Handle,
		)
		reason := strconv.Itoa(resp.StatusCode)
		if env.Message != "" {
			reason += " " + env.Message
		}
		c.discord.Alert(fmt.Sprintf("IG auth failure (%s) on %s for @%s. Job halted.", reason, endpoint, c.session.Handle))
		return nil, fmt.Errorf("%w: %s", ErrIGDenied, reason)

	case resp.StatusCode >= 400:
		c.logRequest(endpoint, resp.StatusCode, elapsed, false, false, errDetail)
		return nil, fmt.Errorf("ig API error: HTTP %d", resp.StatusCode)

	case statusFailed:
		c.logRequest(endpoint, resp.StatusCode, elapsed, false, false, errDetail)
		return nil, fmt.Errorf("ig API error: status %q: %s", env.Status, env.Message)
	}

	c.logRequest(endpoint, resp.StatusCode, elapsed, false, false, "")
	return body, nil
}

// logRequest writes a request log entry to the store.
func (c *HTTPIGClient) logRequest(endpoint string, status int, responseTimeMs int64, throttled, denied bool, errDetail string) {
	entry := &model.IGRequestLog{
		Timestamp:      c.clock.Now(),
		AccountID:      c.session.AccountID,
		Endpoint:       endpoint,
		HTTPStatus:     status,
		IsThrottled:    throttled,
		IsDenied:       denied,
		ResponseTimeMs: responseTimeMs,
		PublicIP:       c.publicIP,
		ErrorDetail:    errDetail,
	}
	if err := c.logStore.Insert(entry); err != nil {
		c.logger.Error("failed to write ig_request_log", "error", err)
	}
}

// parsePost converts a raw feed item into an IGPost.
// Returns nil for posts with no downloadable image (videos, all-video
// carousels, unknown media types, items without image candidates).
func (c *HTTPIGClient) parsePost(item igFeedItem) *model.IGPost {
	post := &model.IGPost{
		ID:        item.ID,
		MediaType: item.MediaType,
		TakenAt:   time.Unix(item.TakenAt, 0).UTC(),
	}

	switch item.MediaType {
	case 1: // Single image.
		if img := selectBestImage(item.ImageVersions2.Candidates); img != nil {
			post.Images = []model.IGImage{
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
			if img := selectBestImage(cm.ImageVersions2.Candidates); img != nil {
				post.Images = append(post.Images, model.IGImage{
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

	if len(post.Images) == 0 {
		c.logger.Info("skipping post with no downloadable images",
			"post_id", item.ID,
			"media_type", item.MediaType,
		)
		return nil
	}
	return post
}

// selectBestImage picks the candidate with the largest area (width*height).
func selectBestImage(candidates []igImageCandidate) *igImageCandidate {
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

// truncate limits a string to maxLen bytes.
func truncate(s string, maxLen int) string {
	if len(s) <= maxLen {
		return s
	}
	return s[:maxLen]
}

// ExtractCSRFToken extracts the csrftoken value from a raw cookie string.
func ExtractCSRFToken(cookies string) string {
	for pair := range strings.SplitSeq(cookies, ";") {
		pair = strings.TrimSpace(pair)
		if strings.HasPrefix(pair, "csrftoken=") {
			return strings.TrimPrefix(pair, "csrftoken=")
		}
	}
	return ""
}
