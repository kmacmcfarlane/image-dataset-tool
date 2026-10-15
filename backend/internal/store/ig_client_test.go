package store_test

import (
	"context"
	"errors"
	"log/slog"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"sync"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/store"
)

// fakeClock is a controllable clock for tests.
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

// fakeSleeper records requested backoff delays without sleeping.
type fakeSleeper struct {
	delays []time.Duration
	err    error
}

func (s *fakeSleeper) Sleep(_ context.Context, d time.Duration) error {
	s.delays = append(s.delays, d)
	return s.err
}

// fakeLogStore records request log inserts in memory.
type fakeLogStore struct {
	logs   []*model.IGRequestLog
	lastIP string
}

func (s *fakeLogStore) Insert(log *model.IGRequestLog) error {
	s.logs = append(s.logs, log)
	return nil
}

func (s *fakeLogStore) LastIPForAccount(_ string) (string, error) {
	return s.lastIP, nil
}

// fakeDiscord records discord messages.
type fakeDiscord struct {
	warnings []string
	alerts   []string
}

func (d *fakeDiscord) Warn(msg string)  { d.warnings = append(d.warnings, msg) }
func (d *fakeDiscord) Alert(msg string) { d.alerts = append(d.alerts, msg) }

// fakeIPResolver returns a fixed IP.
type fakeIPResolver struct {
	ip  string
	err error
}

func (r *fakeIPResolver) Resolve() (string, error) { return r.ip, r.err }

// capturedRecord is one slog record captured by recordingHandler.
type capturedRecord struct {
	msg   string
	attrs map[string]any
}

// recordingHandler is a slog.Handler that keeps every record in memory.
type recordingHandler struct {
	mu      *sync.Mutex
	records *[]capturedRecord
}

func newRecordingHandler() *recordingHandler {
	return &recordingHandler{mu: &sync.Mutex{}, records: &[]capturedRecord{}}
}

func (h *recordingHandler) Enabled(context.Context, slog.Level) bool { return true }
func (h *recordingHandler) WithAttrs([]slog.Attr) slog.Handler       { return h }
func (h *recordingHandler) WithGroup(string) slog.Handler            { return h }
func (h *recordingHandler) Handle(_ context.Context, r slog.Record) error {
	rec := capturedRecord{msg: r.Message, attrs: map[string]any{}}
	r.Attrs(func(a slog.Attr) bool {
		rec.attrs[a.Key] = a.Value.Any()
		return true
	})
	h.mu.Lock()
	defer h.mu.Unlock()
	*h.records = append(*h.records, rec)
	return nil
}

func (h *recordingHandler) find(msg string) []capturedRecord {
	h.mu.Lock()
	defer h.mu.Unlock()
	var out []capturedRecord
	for _, r := range *h.records {
		if r.msg == msg {
			out = append(out, r)
		}
	}
	return out
}

func loadFixture(name string) []byte {
	data, err := os.ReadFile(filepath.Join("testdata", "ig", name))
	Expect(err).NotTo(HaveOccurred())
	return data
}

func validSession(clock *fakeClock) *model.IGSession {
	return &model.IGSession{
		AccountID: "acc-1",
		Handle:    "testuser",
		Cookies:   "sessionid=abc123; csrftoken=tok456; ds_user_id=53611243",
		CSRFToken: "tok456",
		ExpiresAt: clock.Now().Add(24 * time.Hour),
	}
}

func expiredSession(clock *fakeClock) *model.IGSession {
	s := validSession(clock)
	s.ExpiresAt = clock.Now().Add(-1 * time.Hour)
	return s
}

// respondWith returns a handler that always writes status and body.
func respondWith(status int, body []byte) http.HandlerFunc {
	return func(w http.ResponseWriter, _ *http.Request) {
		w.WriteHeader(status)
		_, _ = w.Write(body)
	}
}

var _ = Describe("HTTPIGClient", func() {
	var (
		clock   *fakeClock
		logs    *fakeLogStore
		discord *fakeDiscord
		sleeper *fakeSleeper
		ctx     context.Context
	)

	BeforeEach(func() {
		clock = &fakeClock{now: time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)}
		logs = &fakeLogStore{}
		discord = &fakeDiscord{}
		sleeper = &fakeSleeper{}
		ctx = context.Background()
	})

	// newClient builds a client against ts with fake clock and sleeper.
	newClient := func(ts *httptest.Server, extra ...store.HTTPIGClientOption) *store.HTTPIGClient {
		opts := []store.HTTPIGClientOption{
			store.WithClock(clock),
			store.WithSleeper(sleeper),
			store.WithHTTPClient(ts.Client()),
			store.WithBaseURL(ts.URL),
		}
		return store.NewHTTPIGClient(validSession(clock), logs, discord, append(opts, extra...)...)
	}

	Describe("SessionValid", func() {
		It("returns true for a valid (non-expired) session", func() {
			client := store.NewHTTPIGClient(validSession(clock), logs, discord, store.WithClock(clock))
			valid, err := client.SessionValid()
			Expect(err).NotTo(HaveOccurred())
			Expect(valid).To(BeTrue())
		})

		It("returns false for an expired session", func() {
			client := store.NewHTTPIGClient(expiredSession(clock), logs, discord, store.WithClock(clock))
			valid, err := client.SessionValid()
			Expect(err).NotTo(HaveOccurred())
			Expect(valid).To(BeFalse())
		})

		It("returns ErrNoSession when no cookies are set", func() {
			session := &model.IGSession{AccountID: "acc-1", Handle: "test"}
			client := store.NewHTTPIGClient(session, logs, discord, store.WithClock(clock))
			_, err := client.SessionValid()
			Expect(err).To(MatchError(store.ErrIGNoSession))
		})
	})

	Describe("Session TTL guard", func() {
		It("halts before any API call and sends a Discord alert when the session is expired", func() {
			var hits int
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits++ }))
			defer ts.Close()

			client := store.NewHTTPIGClient(expiredSession(clock), logs, discord,
				store.WithClock(clock), store.WithHTTPClient(ts.Client()), store.WithBaseURL(ts.URL))
			_, err := client.FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})

			Expect(err).To(MatchError(store.ErrIGSessionExpired))
			Expect(hits).To(Equal(0))
			Expect(discord.alerts).To(HaveLen(1))
			Expect(discord.alerts[0]).To(ContainSubstring("session expired"))
			Expect(discord.alerts[0]).To(ContainSubstring("@testuser"))
		})
	})

	Describe("Request construction", func() {
		It("sends correct headers including cookie and csrf token", func() {
			var capturedReq *http.Request
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedReq = r.Clone(r.Context())
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(loadFixture("feed_single_image.json"))
			}))
			defer ts.Close()

			_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())

			Expect(capturedReq.Header.Get("Cookie")).To(Equal(validSession(clock).Cookies))
			Expect(capturedReq.Header.Get("X-CSRFToken")).To(Equal("tok456"))
			Expect(capturedReq.Header.Get("Referer")).To(Equal("https://www.instagram.com/"))
			Expect(capturedReq.Header.Get("X-IG-App-ID")).To(Equal("936619743392459"))
			Expect(capturedReq.Header.Get("X-Requested-With")).To(Equal("XMLHttpRequest"))
			Expect(capturedReq.Header.Get("User-Agent")).NotTo(BeEmpty())
		})

		DescribeTable("builds the feed endpoint URL",
			func(userID string, opts model.IGFetchOpts, wantPath, wantQuery string) {
				var gotPath, gotQuery string
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					gotPath = r.URL.EscapedPath()
					gotQuery = r.URL.RawQuery
					w.WriteHeader(http.StatusOK)
					_, _ = w.Write(loadFixture("feed_single_image.json"))
				}))
				defer ts.Close()

				_, err := newClient(ts).FetchPosts(ctx, userID, opts)
				Expect(err).NotTo(HaveOccurred())
				Expect(gotPath).To(Equal(wantPath))
				Expect(gotQuery).To(Equal(wantQuery))
			},
			Entry("count only", "53611243", model.IGFetchOpts{Count: 5},
				"/api/v1/feed/user/53611243/", "count=5"),
			Entry("default count", "53611243", model.IGFetchOpts{},
				"/api/v1/feed/user/53611243/", "count=12"),
			Entry("with max_id", "53611243", model.IGFetchOpts{Count: 12, MaxID: "2999999999999999999_53611243"},
				"/api/v1/feed/user/53611243/", "count=12&max_id=2999999999999999999_53611243"),
			Entry("escapes max_id", "53611243", model.IGFetchOpts{Count: 12, MaxID: "a&b=c d"},
				"/api/v1/feed/user/53611243/", "count=12&max_id=a%26b%3Dc+d"),
			Entry("escapes user id", "1/../x?y", model.IGFetchOpts{Count: 1},
				"/api/v1/feed/user/1%2F..%2Fx%3Fy/", "count=1"),
		)
	})

	Describe("FetchProfile", func() {
		It("fills username and full name from the first post's owner", func() {
			var gotQuery string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				gotQuery = r.URL.RawQuery
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(loadFixture("feed_single_image.json"))
			}))
			defer ts.Close()

			profile, err := newClient(ts).FetchProfile(ctx, "53611243")
			Expect(err).NotTo(HaveOccurred())
			Expect(profile).To(Equal(&model.IGProfile{
				UserID:   "53611243",
				Username: "example_artist",
				FullName: "Example Artist",
			}))
			Expect(gotQuery).To(Equal("count=1"))
			Expect(logs.logs).To(HaveLen(1))
		})

		It("returns only the user ID when the user has no posts", func() {
			ts := httptest.NewServer(respondWith(http.StatusOK, []byte(`{"items":[],"status":"ok"}`)))
			defer ts.Close()

			profile, err := newClient(ts).FetchProfile(ctx, "53611243")
			Expect(err).NotTo(HaveOccurred())
			Expect(profile).To(Equal(&model.IGProfile{UserID: "53611243"}))
		})

		It("returns ErrDenied on a 403", func() {
			ts := httptest.NewServer(respondWith(http.StatusForbidden, []byte(`{"message":"forbidden"}`)))
			defer ts.Close()

			_, err := newClient(ts).FetchProfile(ctx, "53611243")
			Expect(err).To(MatchError(store.ErrIGDenied))
		})
	})

	Describe("Fixture-based feed parsing", func() {
		It("selects the best resolution (max area) for a single image post", func() {
			ts := httptest.NewServer(respondWith(http.StatusOK, loadFixture("feed_single_image.json")))
			defer ts.Close()

			result, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Posts).To(HaveLen(1))

			post := result.Posts[0]
			Expect(post.MediaType).To(Equal(1))
			Expect(post.Images).To(HaveLen(1))
			Expect(post.Images[0].Width).To(Equal(1080))
			Expect(post.Images[0].Height).To(Equal(1080))
			Expect(post.Images[0].URL).To(ContainSubstring("large.jpg"))

			Expect(result.MoreAvailable).To(BeTrue())
			Expect(result.NextMaxID).NotTo(BeEmpty())
		})

		It("iterates carousel_media, yields one image per image slide and logs the skipped video slide", func() {
			handler := newRecordingHandler()
			ts := httptest.NewServer(respondWith(http.StatusOK, loadFixture("feed_carousel.json")))
			defer ts.Close()

			result, err := newClient(ts, store.WithLogger(slog.New(handler))).
				FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Posts).To(HaveLen(1))

			post := result.Posts[0]
			Expect(post.MediaType).To(Equal(8))
			// 3 carousel items: 2 images + 1 video (skipped)
			Expect(post.Images).To(HaveLen(2))
			Expect(post.Images[0].SlideIndex).To(Equal(0))
			Expect(post.Images[0].Width).To(Equal(1440))
			Expect(post.Images[0].URL).To(ContainSubstring("slide0_large.jpg"))
			Expect(post.Images[1].SlideIndex).To(Equal(1))
			Expect(post.Images[1].Width).To(Equal(1080))

			skips := handler.find("skipping video in carousel")
			Expect(skips).To(HaveLen(1))
			Expect(skips[0].attrs).To(HaveKeyWithValue("slide_index", int64(2)))
			Expect(skips[0].attrs).To(HaveKeyWithValue("reason", "media_type=2 (video)"))
		})

		It("skips video-only posts and logs the skip reason", func() {
			handler := newRecordingHandler()
			ts := httptest.NewServer(respondWith(http.StatusOK, loadFixture("feed_video_only.json")))
			defer ts.Close()

			result, err := newClient(ts, store.WithLogger(slog.New(handler))).
				FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Posts).To(BeEmpty())

			skips := handler.find("skipping video post")
			Expect(skips).To(HaveLen(1))
			Expect(skips[0].attrs).To(HaveKeyWithValue("post_id", "3001234567890123458_53611243"))
			Expect(skips[0].attrs).To(HaveKeyWithValue("reason", "media_type=2 (video)"))
		})

		It("skips a carousel whose slides are all videos", func() {
			body := []byte(`{"items":[{"id":"c1","media_type":8,"taken_at":1716000000,
				"carousel_media":[{"id":"c1_1","media_type":2},{"id":"c1_2","media_type":2}]}],"status":"ok"}`)
			ts := httptest.NewServer(respondWith(http.StatusOK, body))
			defer ts.Close()

			result, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Posts).To(BeEmpty())
		})

		It("returns only image posts and carousel images from a mixed feed", func() {
			ts := httptest.NewServer(respondWith(http.StatusOK, loadFixture("feed_mixed.json")))
			defer ts.Close()

			result, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())

			// 4 items: image, video (skip), carousel, image => 3 posts
			Expect(result.Posts).To(HaveLen(3))
			Expect(result.Posts[0].MediaType).To(Equal(1))
			Expect(result.Posts[1].MediaType).To(Equal(8))
			Expect(result.Posts[1].Images).To(HaveLen(2)) // 2 image slides in carousel
			Expect(result.Posts[2].MediaType).To(Equal(1))
		})
	})

	Describe("Denial handling", func() {
		DescribeTable("returns ErrDenied, logs is_denied=true and sends a Discord alert",
			func(status int, body string, wantInAlert string) {
				ts := httptest.NewServer(respondWith(status, []byte(body)))
				defer ts.Close()

				_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
				Expect(err).To(MatchError(store.ErrIGDenied))

				Expect(logs.logs).To(HaveLen(1))
				Expect(logs.logs[0].IsDenied).To(BeTrue())
				Expect(logs.logs[0].IsThrottled).To(BeFalse())
				Expect(logs.logs[0].HTTPStatus).To(Equal(status))
				Expect(logs.logs[0].ErrorDetail).To(Equal(body))

				Expect(discord.alerts).To(HaveLen(1))
				Expect(discord.alerts[0]).To(ContainSubstring("auth failure"))
				Expect(discord.alerts[0]).To(ContainSubstring(wantInAlert))
				Expect(discord.alerts[0]).To(ContainSubstring("Job halted"))
				Expect(sleeper.delays).To(BeEmpty())
			},
			Entry("401", http.StatusUnauthorized, `{"message":"unauthorized"}`, "401"),
			Entry("403", http.StatusForbidden, `{"message":"forbidden"}`, "403"),
			Entry("400 checkpoint_required", http.StatusBadRequest,
				`{"message":"checkpoint_required","checkpoint_url":"https://i.instagram.com/challenge/","status":"fail"}`,
				"checkpoint_required"),
			Entry("400 challenge_required", http.StatusBadRequest,
				`{"message":"challenge_required","status":"fail"}`, "challenge_required"),
			Entry("200 status=fail login_required", http.StatusOK,
				`{"message":"login_required","status":"fail"}`, "login_required"),
		)

		DescribeTable("treats a redirect to login/challenge as denial instead of following it",
			func(location string) {
				var hits int
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					hits++
					if r.URL.Path != "/api/v1/feed/user/53611243/" {
						w.WriteHeader(http.StatusOK) // The login page would answer 200.
						return
					}
					http.Redirect(w, r, location, http.StatusFound)
				}))
				defer ts.Close()

				_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
				Expect(err).To(MatchError(store.ErrIGDenied))
				Expect(hits).To(Equal(1)) // Redirect not followed.

				Expect(logs.logs).To(HaveLen(1))
				Expect(logs.logs[0].IsDenied).To(BeTrue())
				Expect(logs.logs[0].HTTPStatus).To(Equal(http.StatusFound))
				Expect(logs.logs[0].ErrorDetail).To(ContainSubstring(location))
				Expect(discord.alerts).To(HaveLen(1))
				Expect(discord.alerts[0]).To(ContainSubstring("redirect to " + location))
			},
			Entry("relative login path", "/accounts/login/?next=/api/v1/feed/user/53611243/"),
			Entry("absolute login URL", "https://www.instagram.com/accounts/login/"),
			Entry("challenge", "/challenge/12345/abcde/"),
		)
	})

	Describe("Other API errors", func() {
		It("returns an error and logs error_detail for a 200 with status=fail", func() {
			body := `{"message":"something unexpected","status":"fail"}`
			ts := httptest.NewServer(respondWith(http.StatusOK, []byte(body)))
			defer ts.Close()

			_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).To(MatchError(ContainSubstring(`status "fail"`)))
			Expect(errors.Is(err, store.ErrIGDenied)).To(BeFalse())

			Expect(logs.logs).To(HaveLen(1))
			Expect(logs.logs[0].HTTPStatus).To(Equal(200))
			Expect(logs.logs[0].ErrorDetail).To(Equal(body))
			Expect(discord.alerts).To(BeEmpty())
		})

		It("returns an error for a redirect that is not to login/challenge", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				http.Redirect(w, r, "/somewhere/else/", http.StatusFound)
			}))
			defer ts.Close()

			_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).To(MatchError(ContainSubstring("unexpected redirect")))
			Expect(logs.logs).To(HaveLen(1))
			Expect(logs.logs[0].HTTPStatus).To(Equal(http.StatusFound))
			Expect(logs.logs[0].ErrorDetail).To(Equal("redirect to /somewhere/else/"))
			Expect(discord.alerts).To(BeEmpty())
		})

		It("returns an error for other 4xx/5xx without alerting", func() {
			ts := httptest.NewServer(respondWith(http.StatusInternalServerError, []byte("boom")))
			defer ts.Close()

			_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).To(MatchError(ContainSubstring("HTTP 500")))
			Expect(logs.logs).To(HaveLen(1))
			Expect(logs.logs[0].ErrorDetail).To(Equal("boom"))
			Expect(discord.alerts).To(BeEmpty())
		})

		It("truncates error_detail to 500 bytes", func() {
			long := make([]byte, 800)
			for i := range long {
				long[i] = 'x'
			}
			ts := httptest.NewServer(respondWith(http.StatusInternalServerError, long))
			defer ts.Close()

			_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).To(HaveOccurred())
			Expect(logs.logs[0].ErrorDetail).To(HaveLen(500))
		})

		It("logs a row with http_status 0 when the transport fails", func() {
			ts := httptest.NewServer(respondWith(http.StatusOK, nil))
			client := newClient(ts)
			ts.Close() // Connection refused from here on.

			_, err := client.FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).To(MatchError(ContainSubstring("ig request failed")))

			Expect(logs.logs).To(HaveLen(1))
			Expect(logs.logs[0].HTTPStatus).To(Equal(0))
			Expect(logs.logs[0].AccountID).To(Equal("acc-1"))
			Expect(logs.logs[0].ErrorDetail).NotTo(BeEmpty())
		})
	})

	Describe("429 backoff", func() {
		It("retries with exponential backoff, then returns ErrThrottled with one Discord warning", func() {
			var hits int
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits++
				w.WriteHeader(http.StatusTooManyRequests)
				_, _ = w.Write([]byte(`{"message":"rate limited"}`))
			}))
			defer ts.Close()

			client := newClient(ts, store.WithBackoff(store.IGBackoff{
				MaxRetries: 3, Initial: 10 * time.Second, Max: 25 * time.Second,
			}))
			_, err := client.FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).To(MatchError(store.ErrIGThrottled))

			Expect(hits).To(Equal(4))
			Expect(sleeper.delays).To(Equal([]time.Duration{10 * time.Second, 20 * time.Second, 25 * time.Second}))

			Expect(logs.logs).To(HaveLen(4))
			for _, l := range logs.logs {
				Expect(l.IsThrottled).To(BeTrue())
				Expect(l.IsDenied).To(BeFalse())
				Expect(l.HTTPStatus).To(Equal(429))
			}

			Expect(discord.warnings).To(HaveLen(1))
			Expect(discord.warnings[0]).To(ContainSubstring("429"))
			Expect(discord.alerts).To(BeEmpty())
		})

		DescribeTable("treats rate-limit messages like a 429",
			func(status int, body string) {
				ts := httptest.NewServer(respondWith(status, []byte(body)))
				defer ts.Close()

				_, err := newClient(ts, store.WithBackoff(store.IGBackoff{
					MaxRetries: 1, Initial: time.Second, Max: time.Second,
				})).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
				Expect(err).To(MatchError(store.ErrIGThrottled))

				Expect(sleeper.delays).To(Equal([]time.Duration{time.Second}))
				Expect(logs.logs).To(HaveLen(2))
				Expect(logs.logs[0].IsThrottled).To(BeTrue())
				Expect(logs.logs[0].IsDenied).To(BeFalse())
				Expect(logs.logs[0].HTTPStatus).To(Equal(status))
				Expect(discord.warnings).To(HaveLen(1))
				Expect(discord.alerts).To(BeEmpty())
			},
			Entry("400 feedback_required", http.StatusBadRequest,
				`{"message":"feedback_required","spam":true,"status":"fail"}`),
			Entry("400 please wait", http.StatusBadRequest,
				`{"message":"Please wait a few minutes before you try again.","status":"fail"}`),
		)

		It("succeeds when a retry after backoff returns 200", func() {
			var hits int
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits++
				if hits == 1 {
					w.WriteHeader(http.StatusTooManyRequests)
					return
				}
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write(loadFixture("feed_single_image.json"))
			}))
			defer ts.Close()

			result, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).NotTo(HaveOccurred())
			Expect(result.Posts).To(HaveLen(1))

			Expect(sleeper.delays).To(Equal([]time.Duration{store.DefaultIGBackoff.Initial}))
			Expect(logs.logs).To(HaveLen(2))
			Expect(logs.logs[0].IsThrottled).To(BeTrue())
			Expect(logs.logs[1].IsThrottled).To(BeFalse())
			Expect(discord.warnings).To(HaveLen(1))
		})

		It("stops retrying when the backoff wait is interrupted", func() {
			var hits int
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits++
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer ts.Close()

			sleeper.err = context.Canceled
			_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).To(MatchError(context.Canceled))
			Expect(hits).To(Equal(1))
		})

		It("halts on session expiry reached during backoff", func() {
			var hits int
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				hits++
				w.WriteHeader(http.StatusTooManyRequests)
			}))
			defer ts.Close()

			// Advance the clock past expiry on the first backoff wait.
			advancing := &advancingSleeper{clock: clock, by: 48 * time.Hour}
			client := newClient(ts, store.WithSleeper(advancing))

			_, err := client.FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).To(MatchError(store.ErrIGSessionExpired))
			Expect(hits).To(Equal(1))
			Expect(discord.alerts).To(ContainElement(ContainSubstring("session expired")))
		})
	})

	Describe("Request logging", func() {
		It("logs successful requests to the store", func() {
			ts := httptest.NewServer(respondWith(http.StatusOK, loadFixture("feed_single_image.json")))
			defer ts.Close()

			_, err := newClient(ts).FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())

			Expect(logs.logs).To(HaveLen(1))
			log := logs.logs[0]
			Expect(log.AccountID).To(Equal("acc-1"))
			Expect(log.HTTPStatus).To(Equal(200))
			Expect(log.IsThrottled).To(BeFalse())
			Expect(log.IsDenied).To(BeFalse())
			Expect(log.ErrorDetail).To(BeEmpty())
			Expect(log.Endpoint).To(ContainSubstring("/api/v1/feed/user/53611243/"))
		})

		It("includes public IP in logged requests after ResolveAndCheckIP", func() {
			ts := httptest.NewServer(respondWith(http.StatusOK, loadFixture("feed_single_image.json")))
			defer ts.Close()

			client := newClient(ts, store.WithIPResolver(&fakeIPResolver{ip: "9.8.7.6"}))
			Expect(client.ResolveAndCheckIP()).To(Succeed())

			_, err := client.FetchPosts(ctx, "53611243", model.IGFetchOpts{Count: 1})
			Expect(err).NotTo(HaveOccurred())
			Expect(logs.logs).To(HaveLen(1))
			Expect(logs.logs[0].PublicIP).To(Equal("9.8.7.6"))
		})
	})

	Describe("IP rotation check", func() {
		DescribeTable("compares current IP to last-used IP",
			func(lastIP, currentIP string, wantWarning bool) {
				logs.lastIP = lastIP
				client := store.NewHTTPIGClient(validSession(clock), logs, discord,
					store.WithClock(clock),
					store.WithIPResolver(&fakeIPResolver{ip: currentIP}),
				)

				Expect(client.ResolveAndCheckIP()).To(Succeed())
				if wantWarning {
					Expect(discord.warnings).To(HaveLen(1))
					Expect(discord.warnings[0]).To(ContainSubstring("Same public IP"))
					Expect(discord.warnings[0]).To(ContainSubstring(currentIP))
				} else {
					Expect(discord.warnings).To(BeEmpty())
				}
			},
			Entry("same IP warns", "1.2.3.4", "1.2.3.4", true),
			Entry("different IP passes", "1.2.3.4", "5.6.7.8", false),
			Entry("no previous IP passes", "", "1.2.3.4", false),
		)

		It("proceeds without warning when IP resolution fails", func() {
			client := store.NewHTTPIGClient(validSession(clock), logs, discord,
				store.WithClock(clock),
				store.WithIPResolver(&fakeIPResolver{err: errors.New("offline")}),
			)
			Expect(client.ResolveAndCheckIP()).To(Succeed())
			Expect(discord.warnings).To(BeEmpty())
		})
	})

	Describe("DownloadMedia", func() {
		var tmpDir string

		BeforeEach(func() {
			tmpDir = GinkgoT().TempDir()
		})

		It("downloads a file with Referer header only (no cookies)", func() {
			var capturedHeaders http.Header
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedHeaders = r.Header
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("fake-image-bytes"))
			}))
			defer ts.Close()

			path, err := newClient(ts).DownloadMedia(ctx, ts.URL+"/image.jpg", tmpDir, "test.jpg")
			Expect(err).NotTo(HaveOccurred())
			Expect(path).To(Equal(filepath.Join(tmpDir, "test.jpg")))

			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("fake-image-bytes"))

			Expect(capturedHeaders.Get("Referer")).To(Equal("https://www.instagram.com/"))
			Expect(capturedHeaders.Get("Cookie")).To(BeEmpty())

			entries, err := os.ReadDir(tmpDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(HaveLen(1)) // No leftover temp file.
		})

		It("returns an error and writes nothing on a non-200 response", func() {
			ts := httptest.NewServer(respondWith(http.StatusNotFound, []byte("nope")))
			defer ts.Close()

			_, err := newClient(ts).DownloadMedia(ctx, ts.URL+"/image.jpg", tmpDir, "test.jpg")
			Expect(err).To(MatchError(ContainSubstring("HTTP 404")))

			entries, err := os.ReadDir(tmpDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("removes the partial file when the body is cut short", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) {
				w.Header().Set("Content-Length", "1000")
				w.WriteHeader(http.StatusOK)
				_, _ = w.Write([]byte("partial"))
			}))
			defer ts.Close()

			_, err := newClient(ts).DownloadMedia(ctx, ts.URL+"/image.jpg", tmpDir, "test.jpg")
			Expect(err).To(MatchError(ContainSubstring("write file")))

			entries, err := os.ReadDir(tmpDir)
			Expect(err).NotTo(HaveOccurred())
			Expect(entries).To(BeEmpty())
		})

		It("returns an error when destDir does not exist", func() {
			ts := httptest.NewServer(respondWith(http.StatusOK, []byte("x")))
			defer ts.Close()

			_, err := newClient(ts).DownloadMedia(ctx, ts.URL+"/image.jpg", filepath.Join(tmpDir, "missing"), "test.jpg")
			Expect(err).To(MatchError(ContainSubstring("create file")))
		})

		DescribeTable("rejects filenames that are not plain file names, without a request",
			func(filename string) {
				var hits int
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, _ *http.Request) { hits++ }))
				defer ts.Close()

				_, err := newClient(ts).DownloadMedia(ctx, ts.URL+"/image.jpg", tmpDir, filename)
				Expect(err).To(MatchError(ContainSubstring("invalid filename")))
				Expect(hits).To(Equal(0))
			},
			Entry("parent traversal", "../x.jpg"),
			Entry("nested path", "sub/x.jpg"),
			Entry("backslash", `..\x.jpg`),
			Entry("empty", ""),
			Entry("dot", "."),
			Entry("dot-dot", ".."),
		)
	})

	Describe("ExtractCSRFToken", func() {
		DescribeTable("extracts csrftoken from a cookie string",
			func(cookies, want string) {
				Expect(store.ExtractCSRFToken(cookies)).To(Equal(want))
			},
			Entry("present", "sessionid=abc; csrftoken=mytoken123; ds_user_id=12345", "mytoken123"),
			Entry("missing", "sessionid=abc; ds_user_id=12345", ""),
		)
	})
})

// advancingSleeper moves a fakeClock forward instead of sleeping.
type advancingSleeper struct {
	clock *fakeClock
	by    time.Duration
}

func (s *advancingSleeper) Sleep(_ context.Context, _ time.Duration) error {
	s.clock.now = s.clock.now.Add(s.by)
	return nil
}
