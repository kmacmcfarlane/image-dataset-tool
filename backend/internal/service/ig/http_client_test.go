package ig_test

import (
	"context"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"time"

	. "github.com/onsi/ginkgo/v2"
	. "github.com/onsi/gomega"

	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/model"
	"github.com/kmacmcfarlane/image-dataset-tool/backend/internal/service/ig"
)

// fakeClock is a controllable clock for tests.
type fakeClock struct {
	now time.Time
}

func (c *fakeClock) Now() time.Time { return c.now }

// fakeLogStore records request log inserts in memory.
type fakeLogStore struct {
	logs []*model.IGRequestLog
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

func loadFixture(name string) []byte {
	data, err := os.ReadFile(filepath.Join("testdata", "ig", name))
	Expect(err).NotTo(HaveOccurred())
	return data
}

func validSession(clock *fakeClock) *ig.SessionInfo {
	return &ig.SessionInfo{
		AccountID: "acc-1",
		Handle:    "testuser",
		Cookies:   "sessionid=abc123; csrftoken=tok456; ds_user_id=53611243",
		CSRFToken: "tok456",
		ExpiresAt: clock.Now().Add(24 * time.Hour),
	}
}

func expiredSession(clock *fakeClock) *ig.SessionInfo {
	return &ig.SessionInfo{
		AccountID: "acc-1",
		Handle:    "testuser",
		Cookies:   "sessionid=abc123; csrftoken=tok456; ds_user_id=53611243",
		CSRFToken: "tok456",
		ExpiresAt: clock.Now().Add(-1 * time.Hour),
	}
}

var _ = Describe("HTTPIGClient", func() {
	var (
		clock   *fakeClock
		store   *fakeLogStore
		discord *fakeDiscord
		ctx     context.Context
	)

	BeforeEach(func() {
		clock = &fakeClock{now: time.Date(2026, 5, 20, 12, 0, 0, 0, time.UTC)}
		store = &fakeLogStore{}
		discord = &fakeDiscord{}
		ctx = context.Background()
	})

	Describe("SessionValid", func() {
		It("returns true for a valid (non-expired) session", func() {
			client := ig.NewHTTPIGClient(validSession(clock), store, discord, ig.WithClock(clock))
			valid, err := client.SessionValid()
			Expect(err).NotTo(HaveOccurred())
			Expect(valid).To(BeTrue())
		})

		It("returns false for an expired session", func() {
			client := ig.NewHTTPIGClient(expiredSession(clock), store, discord, ig.WithClock(clock))
			valid, err := client.SessionValid()
			Expect(err).NotTo(HaveOccurred())
			Expect(valid).To(BeFalse())
		})

		It("returns ErrNoSession when no cookies are set", func() {
			session := &ig.SessionInfo{AccountID: "acc-1", Handle: "test"}
			client := ig.NewHTTPIGClient(session, store, discord, ig.WithClock(clock))
			_, err := client.SessionValid()
			Expect(err).To(MatchError(ig.ErrNoSession))
		})
	})

	Describe("Session TTL guard", func() {
		It("halts before API call when session is expired", func() {
			client := ig.NewHTTPIGClient(expiredSession(clock), store, discord, ig.WithClock(clock))
			_, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 1})
			Expect(err).To(MatchError(ig.ErrSessionExpired))
		})

		It("sends Discord alert on session expiry", func() {
			client := ig.NewHTTPIGClient(expiredSession(clock), store, discord, ig.WithClock(clock))
			_, _ = client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 1})
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
				w.Write(loadFixture("feed_single_image.json"))
			}))
			defer ts.Close()

			session := validSession(clock)
			client := ig.NewHTTPIGClient(session, store, discord,
				ig.WithClock(clock),
				ig.WithHTTPClient(ts.Client()),
				ig.WithBaseURL(ts.URL),
			)

			_, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())

			Expect(capturedReq.Header.Get("Cookie")).To(Equal(session.Cookies))
			Expect(capturedReq.Header.Get("X-CSRFToken")).To(Equal("tok456"))
			Expect(capturedReq.Header.Get("Referer")).To(Equal("https://www.instagram.com/"))
			Expect(capturedReq.Header.Get("X-IG-App-ID")).To(Equal("936619743392459"))
			Expect(capturedReq.Header.Get("X-Requested-With")).To(Equal("XMLHttpRequest"))
			Expect(capturedReq.Header.Get("User-Agent")).NotTo(BeEmpty())
		})

		It("constructs correct feed endpoint URL", func() {
			var capturedPath string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.RequestURI()
				w.WriteHeader(http.StatusOK)
				w.Write(loadFixture("feed_single_image.json"))
			}))
			defer ts.Close()

			client := ig.NewHTTPIGClient(validSession(clock), store, discord,
				ig.WithClock(clock),
				ig.WithHTTPClient(ts.Client()),
				ig.WithBaseURL(ts.URL),
			)

			_, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 5})
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedPath).To(Equal("/api/v1/feed/user/53611243/?count=5"))
		})

		It("includes max_id for pagination", func() {
			var capturedPath string
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedPath = r.URL.RequestURI()
				w.WriteHeader(http.StatusOK)
				w.Write(loadFixture("feed_single_image.json"))
			}))
			defer ts.Close()

			client := ig.NewHTTPIGClient(validSession(clock), store, discord,
				ig.WithClock(clock),
				ig.WithHTTPClient(ts.Client()),
				ig.WithBaseURL(ts.URL),
			)

			_, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{
				Count: 12,
				MaxID: "2999999999999999999_53611243",
			})
			Expect(err).NotTo(HaveOccurred())
			Expect(capturedPath).To(ContainSubstring("max_id=2999999999999999999_53611243"))
		})
	})

	Describe("Fixture-based feed parsing", func() {
		Context("single image post", func() {
			It("selects the best resolution (max area)", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write(loadFixture("feed_single_image.json"))
				}))
				defer ts.Close()

				client := ig.NewHTTPIGClient(validSession(clock), store, discord,
					ig.WithClock(clock),
					ig.WithHTTPClient(ts.Client()),
					ig.WithBaseURL(ts.URL),
				)

				result, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 12})
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
		})

		Context("carousel post (media_type=8)", func() {
			It("iterates carousel_media and yields multiple images", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write(loadFixture("feed_carousel.json"))
				}))
				defer ts.Close()

				client := ig.NewHTTPIGClient(validSession(clock), store, discord,
					ig.WithClock(clock),
					ig.WithHTTPClient(ts.Client()),
					ig.WithBaseURL(ts.URL),
				)

				result, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 12})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Posts).To(HaveLen(1))

				post := result.Posts[0]
				Expect(post.MediaType).To(Equal(8))
				// 3 carousel items: 2 images + 1 video (skipped)
				Expect(post.Images).To(HaveLen(2))

				// First slide: best resolution selected
				Expect(post.Images[0].SlideIndex).To(Equal(0))
				Expect(post.Images[0].Width).To(Equal(1440))
				Expect(post.Images[0].URL).To(ContainSubstring("slide0_large.jpg"))

				// Second slide
				Expect(post.Images[1].SlideIndex).To(Equal(1))
				Expect(post.Images[1].Width).To(Equal(1080))
			})
		})

		Context("video post (media_type=2)", func() {
			It("skips video-only posts", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write(loadFixture("feed_video_only.json"))
				}))
				defer ts.Close()

				client := ig.NewHTTPIGClient(validSession(clock), store, discord,
					ig.WithClock(clock),
					ig.WithHTTPClient(ts.Client()),
					ig.WithBaseURL(ts.URL),
				)

				result, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 12})
				Expect(err).NotTo(HaveOccurred())
				Expect(result.Posts).To(BeEmpty())
			})
		})

		Context("mixed feed (images + video + carousel)", func() {
			It("returns only image posts and carousel images, skipping videos", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusOK)
					w.Write(loadFixture("feed_mixed.json"))
				}))
				defer ts.Close()

				client := ig.NewHTTPIGClient(validSession(clock), store, discord,
					ig.WithClock(clock),
					ig.WithHTTPClient(ts.Client()),
					ig.WithBaseURL(ts.URL),
				)

				result, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 12})
				Expect(err).NotTo(HaveOccurred())

				// 4 items: image, video (skip), carousel, image => 3 posts
				Expect(result.Posts).To(HaveLen(3))
				Expect(result.Posts[0].MediaType).To(Equal(1))
				Expect(result.Posts[1].MediaType).To(Equal(8))
				Expect(result.Posts[1].Images).To(HaveLen(2)) // 2 image slides in carousel
				Expect(result.Posts[2].MediaType).To(Equal(1))
			})
		})
	})

	Describe("Error path handling", func() {
		Context("429 response", func() {
			It("returns ErrThrottled, logs is_throttled=true, sends Discord warning", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusTooManyRequests)
					w.Write([]byte(`{"message": "rate limited"}`))
				}))
				defer ts.Close()

				client := ig.NewHTTPIGClient(validSession(clock), store, discord,
					ig.WithClock(clock),
					ig.WithHTTPClient(ts.Client()),
					ig.WithBaseURL(ts.URL),
				)

				_, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 1})
				Expect(err).To(MatchError(ig.ErrThrottled))

				Expect(store.logs).To(HaveLen(1))
				Expect(store.logs[0].IsThrottled).To(BeTrue())
				Expect(store.logs[0].HTTPStatus).To(Equal(429))

				Expect(discord.warnings).To(HaveLen(1))
				Expect(discord.warnings[0]).To(ContainSubstring("429"))
			})
		})

		Context("401 response", func() {
			It("returns ErrDenied, logs is_denied=true, sends Discord alert", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusUnauthorized)
					w.Write([]byte(`{"message": "unauthorized"}`))
				}))
				defer ts.Close()

				client := ig.NewHTTPIGClient(validSession(clock), store, discord,
					ig.WithClock(clock),
					ig.WithHTTPClient(ts.Client()),
					ig.WithBaseURL(ts.URL),
				)

				_, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 1})
				Expect(err).To(MatchError(ig.ErrDenied))

				Expect(store.logs).To(HaveLen(1))
				Expect(store.logs[0].IsDenied).To(BeTrue())
				Expect(store.logs[0].HTTPStatus).To(Equal(401))

				Expect(discord.alerts).To(HaveLen(1))
				Expect(discord.alerts[0]).To(ContainSubstring("auth failure"))
			})
		})

		Context("403 response", func() {
			It("returns ErrDenied, logs is_denied=true, sends Discord alert", func() {
				ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
					w.WriteHeader(http.StatusForbidden)
					w.Write([]byte(`{"message": "forbidden"}`))
				}))
				defer ts.Close()

				client := ig.NewHTTPIGClient(validSession(clock), store, discord,
					ig.WithClock(clock),
					ig.WithHTTPClient(ts.Client()),
					ig.WithBaseURL(ts.URL),
				)

				_, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 1})
				Expect(err).To(MatchError(ig.ErrDenied))

				Expect(store.logs).To(HaveLen(1))
				Expect(store.logs[0].IsDenied).To(BeTrue())

				Expect(discord.alerts).To(HaveLen(1))
			})
		})
	})

	Describe("Request logging", func() {
		It("logs successful requests to the store", func() {
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write(loadFixture("feed_single_image.json"))
			}))
			defer ts.Close()

			client := ig.NewHTTPIGClient(validSession(clock), store, discord,
				ig.WithClock(clock),
				ig.WithHTTPClient(ts.Client()),
				ig.WithBaseURL(ts.URL),
			)

			_, err := client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 12})
			Expect(err).NotTo(HaveOccurred())

			Expect(store.logs).To(HaveLen(1))
			log := store.logs[0]
			Expect(log.AccountID).To(Equal("acc-1"))
			Expect(log.HTTPStatus).To(Equal(200))
			Expect(log.IsThrottled).To(BeFalse())
			Expect(log.IsDenied).To(BeFalse())
			Expect(log.Endpoint).To(ContainSubstring("/api/v1/feed/user/53611243/"))
		})
	})

	Describe("IP rotation check", func() {
		It("warns when current IP matches last-used IP", func() {
			store.lastIP = "1.2.3.4"
			resolver := &fakeIPResolver{ip: "1.2.3.4"}

			client := ig.NewHTTPIGClient(validSession(clock), store, discord,
				ig.WithClock(clock),
				ig.WithIPResolver(resolver),
			)

			err := client.ResolveAndCheckIP()
			Expect(err).NotTo(HaveOccurred())

			Expect(discord.warnings).To(HaveLen(1))
			Expect(discord.warnings[0]).To(ContainSubstring("Same public IP"))
			Expect(discord.warnings[0]).To(ContainSubstring("1.2.3.4"))
		})

		It("does not warn when IP is different", func() {
			store.lastIP = "1.2.3.4"
			resolver := &fakeIPResolver{ip: "5.6.7.8"}

			client := ig.NewHTTPIGClient(validSession(clock), store, discord,
				ig.WithClock(clock),
				ig.WithIPResolver(resolver),
			)

			err := client.ResolveAndCheckIP()
			Expect(err).NotTo(HaveOccurred())
			Expect(discord.warnings).To(BeEmpty())
		})

		It("does not warn when no previous IP exists", func() {
			store.lastIP = ""
			resolver := &fakeIPResolver{ip: "1.2.3.4"}

			client := ig.NewHTTPIGClient(validSession(clock), store, discord,
				ig.WithClock(clock),
				ig.WithIPResolver(resolver),
			)

			err := client.ResolveAndCheckIP()
			Expect(err).NotTo(HaveOccurred())
			Expect(discord.warnings).To(BeEmpty())
		})
	})

	Describe("DownloadMedia", func() {
		It("downloads a file with Referer header only (no cookies)", func() {
			var capturedHeaders http.Header
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				capturedHeaders = r.Header
				w.WriteHeader(http.StatusOK)
				w.Write([]byte("fake-image-bytes"))
			}))
			defer ts.Close()

			tmpDir := GinkgoT().TempDir()
			client := ig.NewHTTPIGClient(validSession(clock), store, discord,
				ig.WithClock(clock),
				ig.WithHTTPClient(ts.Client()),
			)

			path, err := client.DownloadMedia(ctx, ts.URL+"/image.jpg", tmpDir, "test.jpg")
			Expect(err).NotTo(HaveOccurred())

			data, err := os.ReadFile(path)
			Expect(err).NotTo(HaveOccurred())
			Expect(string(data)).To(Equal("fake-image-bytes"))

			// Verify Referer is set but no Cookie header
			Expect(capturedHeaders.Get("Referer")).To(Equal("https://www.instagram.com/"))
			Expect(capturedHeaders.Get("Cookie")).To(BeEmpty())
		})
	})

	Describe("ExtractCSRFToken", func() {
		It("extracts csrftoken from cookie string", func() {
			cookies := "sessionid=abc; csrftoken=mytoken123; ds_user_id=12345"
			Expect(ig.ExtractCSRFToken(cookies)).To(Equal("mytoken123"))
		})

		It("returns empty string when csrftoken is missing", func() {
			cookies := "sessionid=abc; ds_user_id=12345"
			Expect(ig.ExtractCSRFToken(cookies)).To(BeEmpty())
		})
	})

	Describe("Public IP in request logs", func() {
		It("includes public IP in logged requests after ResolveAndCheckIP", func() {
			resolver := &fakeIPResolver{ip: "9.8.7.6"}
			ts := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
				w.WriteHeader(http.StatusOK)
				w.Write(loadFixture("feed_single_image.json"))
			}))
			defer ts.Close()

			client := ig.NewHTTPIGClient(validSession(clock), store, discord,
				ig.WithClock(clock),
				ig.WithHTTPClient(ts.Client()),
				ig.WithBaseURL(ts.URL),
				ig.WithIPResolver(resolver),
			)

			err := client.ResolveAndCheckIP()
			Expect(err).NotTo(HaveOccurred())

			_, err = client.FetchPosts(ctx, "53611243", ig.FetchOpts{Count: 1})
			Expect(err).NotTo(HaveOccurred())

			Expect(store.logs).To(HaveLen(1))
			Expect(store.logs[0].PublicIP).To(Equal("9.8.7.6"))
		})
	})
})
