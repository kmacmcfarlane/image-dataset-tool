# IG Client Approach Evaluation — S-013 Research Spike

**Date**: 2026-05-11 (autonomous), 2026-05-20 (interactive PoC)
**Status**: Complete — recommendation revised after PoC

## 1. Executive Summary

**Recommendation: Go HTTP client (`net/http`) with browser session cookie import.**

> **Note:** The autonomous phase (Section 5) originally recommended instaloader.
> The interactive PoC (Section 9) reversed that recommendation after testing
> revealed TLS fingerprint detection risks, endpoint deprecation (429 on
> `web_profile_info`), and a simpler alternative via Instagram's REST feed API.
> Section 9 is authoritative.

A thin Go HTTP client replaying browser session cookies is safer and simpler than
any third-party IG library. The user logs into a burner account in a private
browser window over VPN, exports cookies via devtools, and pastes them into the
app. The Go backend replays these cookies against Instagram's stable REST feed
endpoint (`/api/v1/feed/user/{id}/`), with session TTL enforcement, throttle
logging, public IP tracking, and Discord alerts on expiry or denial.

No Python dependency. No subprocess. No TLS fingerprint mismatch. The `IGClient`
interface allows swapping implementations if the REST endpoint is deprecated.

## 2. Evaluation Matrix

| Criterion              | Instaloader (Python) | Go HTTP Client (goinsta) | Node Sidecar (instagram-private-api) | GraphQL Scraping (raw) | Official Graph API | Browser Automation (Playwright) |
|------------------------|---------------------|--------------------------|--------------------------------------|------------------------|--------------------|---------------------------------|
| **Auth flow**          | Session file + login; cookie import from browser | Login + session export; chromedp for challenges | Login + state serialization | Requires session cookie + x-ig-app-id header | OAuth 2.0 (Facebook Login) | Full browser login |
| **Media types**        | Posts, carousels, stories, reels, highlights, IGTV | Posts, carousels, stories | Posts, carousels, stories, reels | Posts, carousels (endpoint-dependent) | Own account only | All visible content |
| **Rate limit handling**| Built-in retry on 429 with backoff; custom RateController API | Basic; user-managed | Basic retry logic | Manual implementation | 200 calls/hr/account | N/A (browser-speed limited) |
| **Maintenance status** | Active (v4.15.1, Apr 2026 commits) | Abandoned (last publish Jan 2023) | Unmaintained (last publish ~2024, 2yr stale) | N/A (DIY) | Maintained by Meta | Maintained by Microsoft |
| **Go integration**     | Subprocess (exec.Command) | Native Go import | HTTP sidecar or subprocess | Native Go net/http | Native Go net/http | Subprocess (heavy) |
| **2FA: TOTP**          | Yes (two_factor_login) | Yes (via chromedp challenge solver) | Yes (verificationMethod '0') | Manual (pass cookie) | N/A (OAuth) | Yes (manual entry in browser) |
| **2FA: SMS**           | Yes | Yes | Yes (verificationMethod '1') | Manual | N/A | Yes |
| **2FA: Email**         | Yes (checkpoint challenge flow) | Partial (chromedp) | Not documented | Manual | N/A | Yes |
| **Carousel support**   | Native (sidecar_nodes iteration) | Yes | Yes | Yes (GraphQL edges) | Own account only | Yes |
| **License**            | MIT | MIT | MIT | N/A | Proprietary API | Apache 2.0 |

## 3. Detailed Analysis

### 3.1 Instaloader (Python subprocess) — ORIGINALLY RECOMMENDED, SUPERSEDED BY SECTION 9

**Repository**: [instaloader/instaloader](https://github.com/instaloader/instaloader)
**Latest release**: v4.15.1 (commits through Apr 2026)
**PyPI**: `pip install instaloader`

**Strengths:**
- Most actively maintained IG scraping tool in the ecosystem (8+ years of development)
- Covers all media types: posts, carousels (sidecars), stories, highlights, reels, IGTV
- Built-in 2FA support (TOTP, SMS, checkpoint challenge for email)
- Session file persistence — login once, reuse session indefinitely (sessions rarely expire)
- `--quiet` flag for headless/automated operation (no interactive prompts)
- Custom `RateController` API allows fine-grained rate limit tuning
- Rich Python module API for programmatic use beyond CLI
- Metadata export (captions, timestamps, geotags) alongside media
- `--load-cookies` option imports session from browser (useful for initial auth)

**Weaknesses:**
- Subprocess overhead: each invocation spawns a Python process
- Cross-language boundary: error handling requires parsing stderr/exit codes
- Recent 429 issues (Feb 2026) with "useragent mismatch" errors — requires staying on latest version
- Instagram's anti-bot measures are increasingly aggressive; even instaloader users hit rate limits with conservative settings
- Not a Go-native dependency — adds Python to the container requirements

**Go integration pattern:**
```
Go worker (media.fetch.instagram consumer)
  → exec.Command("python3", "-m", "instaloader", "--sessionfile", path, "--quiet", ...)
  → parse stdout/stderr for progress, errors
  → downloaded files land in $DATA_DIR/tmp/<job-id>/
  → Go worker moves to processing pipeline
```

Session management: Go stores encrypted session file path in SQLite. On first
login, user authenticates via UI (interactive phase). Subsequent runs reuse the
session file. Session refresh handled by instaloader internally.

**Rate limiting integration:**
The Go worker's `time.Ticker` pacer (2s default from PRD config) controls invocation
frequency. Instaloader's internal rate controller handles per-request throttling.
On 429, instaloader retries internally; if it fails after max attempts, the Go
worker receives a non-zero exit code and NAKs with backoff.

### 3.2 Go HTTP Client (goinsta)

**Repository**: [Davincible/goinsta](https://github.com/Davincible/goinsta) (fork of ahmdrz/goinsta)
**Latest publish**: January 2023
**Status**: Effectively abandoned

**Strengths:**
- Native Go — no subprocess overhead, type-safe integration
- Direct import as Go module (`github.com/Davincible/goinsta/v3`)
- Uses chromedp for challenge/checkpoint solving

**Weaknesses:**
- Last package publish was January 2023 — over 3 years stale
- Instagram changes private API endpoints frequently; a library dormant since 2023 is almost certainly broken against current IG
- Original repo (ahmdrz/goinsta) archived; Davincible fork has open issues but no recent fixes
- chromedp dependency adds browser engine weight to the container
- No community keeping up with IG protocol changes
- Risk of immediate breakage with no path to fix except forking and maintaining ourselves

**Verdict**: Not viable. The maintenance gap is too large for a target (Instagram's private API) that changes constantly.

### 3.3 Node Sidecar (instagram-private-api)

**Repository**: [dilame/instagram-private-api](https://github.com/dilame/instagram-private-api)
**Latest npm publish**: v1.46.1 (~2024, 2 years stale)
**Status**: Unmaintained

**Strengths:**
- TypeScript SDK with good type definitions
- Supports 2FA (TOTP via method '0', SMS via method '1')
- State serialization for session persistence
- Posts, carousels, stories, reels support

**Weaknesses:**
- Last published 2+ years ago — same staleness problem as goinsta
- Instagram private API changes break unmaintained clients
- Adds Node.js runtime dependency and sidecar process to the stack
- Integration requires HTTP API bridge or subprocess — similar complexity to instaloader but less mature
- Recent GitHub issues (2026) report connection failures
- No active maintainer responding to issues

**Verdict**: Not viable. Same maintenance problem as goinsta, plus adds Node.js as an additional runtime dependency with no offsetting benefit over instaloader.

### 3.4 GraphQL Scraping (raw HTTP)

**Description**: Direct HTTP requests to Instagram's GraphQL and REST endpoints using Go's `net/http`.

**Strengths:**
- Native Go — no external dependencies
- Full control over request/response handling
- No third-party library to break

**Weaknesses:**
- Instagram changes `doc_id` parameters every 2-4 weeks (as of 2026)
- Requires `x-ig-app-id` header and authenticated session
- Anti-bot detection is aggressive: TLS fingerprinting, header order analysis, canvas fingerprinting
- Building and maintaining a scraper from scratch means constant reverse-engineering work
- The PRD explicitly prohibits "unofficial scraping/spoofing/reverse-engineered APIs" (DEVELOPMENT_PRACTICES section 8)
- No community support — you're on your own when endpoints change

**Verdict**: Not viable. Violates DEVELOPMENT_PRACTICES section 8 (no reverse-engineered APIs), and the maintenance burden of tracking endpoint changes is unsustainable for a single-developer project.

### 3.5 Official Instagram Graph API

**Documentation**: [Meta Developer Docs](https://developers.facebook.com/docs/instagram-platform/)
**Status**: Actively maintained by Meta

**Strengths:**
- Official, stable, documented API
- OAuth 2.0 authentication (no credential scraping)
- No TOS violation risk
- Rate limits clearly documented (200 calls/hr/account)

**Weaknesses:**
- **Cannot read other users' content** — only returns data for accounts you own/manage
- Requires Business or Creator account (personal accounts have no API access since Basic Display API deprecation Dec 2024)
- Designed for social media management tools, not media collection
- Cannot access: other users' posts, public hashtag streams, follower lists, or discovery data
- Would require the target IG accounts to grant your app permission — not feasible for the use case

**Verdict**: Not viable for this project's use case. The Graph API is designed for managing your own accounts, not for collecting media from other users' profiles.

### 3.6 Browser Automation (Playwright)

**Description**: Full browser automation using Playwright (Node.js/Python) to navigate Instagram as a real user.

**Strengths:**
- Most resilient to API changes (renders actual pages)
- Handles any content type visible in the browser
- Full 2FA support (user can interact with challenges)
- Hardest for Instagram to distinguish from real users

**Weaknesses:**
- 10x slower than HTTP-based approaches
- Heavy resource footprint (full Chromium instance per session)
- Instagram actively fingerprints headless browsers (canvas, WebGL, navigator properties)
- Fragile selectors — UI changes break automation
- Complex session management
- Adds ~400MB+ to container size for browser binaries
- The PRD specifies `claude-sandbox` Docker environment — adding a full browser is heavyweight

**Verdict**: Not recommended as primary approach. Could serve as a fallback for initial authentication/2FA challenge solving, but too slow and resource-heavy for media fetching at scale.

## 4. 2FA Support Matrix

| Method | Instaloader | goinsta | instagram-private-api | GraphQL (raw) | Graph API | Playwright |
|--------|-------------|---------|----------------------|---------------|-----------|------------|
| **TOTP (Authenticator app)** | Yes — `two_factor_login()` method | Yes — chromedp challenge solver | Yes — verificationMethod '0' | Manual cookie injection | N/A (OAuth) | Yes — manual UI entry |
| **SMS** | Yes — prompted during login flow | Yes — chromedp | Yes — verificationMethod '1' (default) | Manual cookie injection | N/A | Yes — manual UI entry |
| **Email (checkpoint)** | Yes — checkpoint challenge URL flow | Partial — chromedp may handle | Not documented | Manual | N/A | Yes — manual UI entry |

**Notes on instaloader 2FA:**
- TOTP is supported but some users report intermittent "code no longer valid" errors (timing sensitivity)
- The recommended workflow: perform initial login with 2FA interactively, then reuse the session file for all automated runs
- `--load-cookies` from browser provides an alternative path that bypasses the 2FA flow entirely

## 5. Original Recommendation (Autonomous Phase — SUPERSEDED)

> **⚠️ This section is superseded by Section 9 (Interactive PoC).** Kept for
> historical context. The revised recommendation is a Go HTTP client with
> browser cookie import — see Section 9.1.

### Original: Instaloader via subprocess

**Justification:**
1. **Only actively maintained option** — v4.15.1 with commits through Apr 2026
2. **Complete media type coverage** — posts, carousels, stories, reels, highlights
3. **Proven 2FA support** — TOTP, SMS, email checkpoint
4. **Session persistence** — login once, reuse indefinitely
5. **Clean integration boundary** — Go handles jobs/queue/rate-limiting, Python handles IG protocol
6. **Large community** — issues get addressed, endpoint changes tracked
7. **Aligned with PRD constraints** — uses a well-known open-source tool rather than reverse-engineered API

**Integration architecture:**
```
┌─────────────────────────────────────┐
│  Go Backend                         │
│  ┌───────────────┐                  │
│  │ IGClient      │ (interface)      │
│  │ interface      │                  │
│  └───────┬───────┘                  │
│          │                          │
│  ┌───────┴───────┐                  │
│  │ InstaloaderIG │ (implementation) │
│  │ Client        │                  │
│  │               │                  │
│  │ - exec.Command│                  │
│  │ - parse output│                  │
│  │ - session mgmt│                  │
│  └───────┬───────┘                  │
│          │                          │
└──────────┼──────────────────────────┘
           │ subprocess
┌──────────┴──────────────────────────┐
│  Python (instaloader)               │
│  - IG protocol handling             │
│  - Rate limit retry (internal)      │
│  - Media download to filesystem     │
│  - Session file read/write          │
└─────────────────────────────────────┘
```

**Container changes required:**
- Add `python3` and `pip install instaloader` to `Dockerfile.claude-sandbox` or backend Dockerfile
- Session files stored in `$DATA_DIR/sessions/` (encrypted path reference in SQLite)

**IGClient interface sketch (for IG-002):**
```go
type IGClient interface {
    // FetchProfile returns profile metadata (post count, bio, etc.)
    FetchProfile(ctx context.Context, handle string) (*IGProfile, error)

    // FetchPosts returns an iterator over posts from a profile
    FetchPosts(ctx context.Context, handle string, opts FetchOpts) (PostIterator, error)

    // DownloadMedia downloads media files for a post to the given directory
    DownloadMedia(ctx context.Context, post *IGPost, destDir string) ([]DownloadedFile, error)

    // SessionValid checks if the current session is still authenticated
    SessionValid(ctx context.Context) (bool, error)
}
```

### Fallback considerations

If instaloader becomes unmaintained or Instagram blocks it permanently:
1. **Short-term**: Browser cookie import (`--load-cookies`) provides resilience against login-flow changes
2. **Medium-term**: The `IGClient` interface allows swapping implementations without touching the service layer
3. **Long-term**: Playwright-based fallback could be implemented behind the same interface for authentication challenges

## 6. Downstream Story Impact (Autonomous Phase — SUPERSEDED)

> **⚠️ Superseded by Section 9.6.** Kept for historical context.

| Story | Title | Original Impact |
|-------|-------|--------|
| **S-014** | IGClient interface + implementation | ~~Can proceed with instaloader subprocess implementation.~~ → Go HTTP client, see 9.6. |
| **S-015** | Media pull job: fetch → NATS pipeline | ~~Design around instaloader's output format.~~ → REST feed API, see 9.6. |
| **S-016** | Sample writer: post-processing, dedup, thumbnails | No change — works with downloaded files regardless of source. |
| **S-011** | Account management: CRUD social media accounts | ~~Session file per account.~~ → Cookie import via UI, see 9.6. |

## 7. Risks and Mitigations (Autonomous Phase — SUPERSEDED)

> **⚠️ Superseded by Section 9.7.** Kept for historical context.

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| Instagram blocks instaloader | Medium | High | `IGClient` interface allows swap; browser cookie import as fallback auth; large community tracks changes quickly |
| 429 rate limits during bulk fetch | High | Medium | Go worker pacer (2s interval from config); instaloader internal retry; NAK with backoff on failure; configurable `request_interval_ms` |
| Session expiry during long jobs | Low | Medium | Instaloader sessions rarely expire; monitor for auth errors and prompt re-login via UI |
| Python subprocess overhead | Low | Low | One process per fetch batch, not per image; amortized over download time which dominates |
| 2FA TOTP timing issues | Medium | Low | Use session file reuse to avoid repeated 2FA; browser cookie import as alternative |
| Instagram TOS enforcement | Medium | Medium | Tool is for personal use (single user, local); not redistributing data; using well-known open-source tool; accounts should be user's own or have consent |
| Instaloader version breaking change | Low | Medium | Pin version in requirements; test before upgrading; `IGClient` interface isolates impact |

## 8. Legal/TOS Considerations

Instagram's Terms of Service prohibit automated scraping and data collection. Key points:

- **TOS violation**: Using any automated tool to access Instagram content (including instaloader) technically violates Instagram's Terms of Use
- **Account risk**: Accounts used for automated access may be suspended or banned
- **DMCA risk**: Meta has issued takedown notices to developers of unofficial API libraries in the past
- **Mitigating factors for this project**:
  - Single-user, local-only tool (not a service)
  - User pulls their own followed/public content
  - No redistribution of scraped data
  - Rate limiting mimics human browsing patterns
  - Well-established open-source tool (instaloader) rather than custom reverse-engineering

**Recommendation**: Use a dedicated/burner account for automated fetching. Do not use a primary personal account. Accept the risk of account suspension as part of the tool's operating model.

## 9. Interactive Phase — PoC Results (2026-05-20)

### 9.1 Revised Recommendation: Go HTTP Client with Browser Session Import

**The autonomous phase recommended instaloader. The PoC phase reverses that recommendation.**

After hands-on testing, a thin Go HTTP client replaying browser session cookies is
safer and simpler than instaloader. The reasons:

| Factor | Instaloader | Go HTTP + Browser Cookies |
|--------|-------------|--------------------------|
| TLS fingerprint | Python `urllib` — trivially detectable | curl/Go `net/http` — not identical to browser but session cookies already trusted |
| Request pattern | Predictable API-walk pattern | Mirrors browser session; same endpoints the browser hits |
| Telemetry headers | Missing `x-fb-lsd`, `__dyn`, `__csr`, `__spin_*` | Can replay full header set from devtools export |
| `web_profile_info` endpoint | Returns 429 immediately (Feb 2026 breakage) | Not used — REST feed API works |
| Maintenance burden | Tracks IG endpoint changes via community | We control the client; stable REST endpoint |
| Auth flow | Python login or `--load-cookies` | User pastes cookie jar from browser devtools (no programmatic login) |
| Container deps | Adds Python runtime | Zero additional deps (Go `net/http`) |

### 9.2 PoC Execution

**Target**: @mear_one (public profile, street artist)
**Account**: Dedicated burner account in private browser window (separate `ig_did` from primary)
**VPN**: Active on host

**Steps performed:**
1. Loaded profile page (`GET /mear_one/`) — HTTP 200, extracted `user_id: 53611243`
2. Hit REST feed endpoint (`GET /api/v1/feed/user/53611243/?count=12`) — HTTP 200, 12 posts returned
3. Downloaded 5 carousel cover images to `./tmp/mear_one/` — all valid JPEG, 144KB–1.6MB

**Endpoints used:**
- `GET /api/v1/feed/user/{user_id}/?count={n}` — returns posts with `image_versions2.candidates[]`
- CDN URLs (`scontent-*.cdninstagram.com`) — no auth required, just `Referer` header

**What worked:**
- REST feed endpoint is stable (no `doc_id` rotation like GraphQL)
- Response includes `image_versions2.candidates[]` with multiple resolutions per image
- Carousel posts (`media_type=8`) have `carousel_media[]` sub-items
- `media_type` enum: 1=image, 2=video, 8=carousel
- CDN downloads need no session cookies — plain GET with referer

**What didn't work:**
- `GET /api/v1/users/web_profile_info/?username=X` — returns 429 immediately (deprecated endpoint)
- Profile page HTML contains no embedded post data (fully SPA, posts load via separate API calls)

### 9.3 Session & Cookie Architecture

**Cookie lifetimes** (observed from `Set-Cookie` response headers):

| Cookie | Max-Age | Purpose |
|--------|---------|---------|
| `sessionid` | ~90 days (inferred from `ds_user_id`) | Core auth token |
| `ds_user_id` | 7,776,000s (~90 days) | User identity |
| `csrftoken` | 34,560,000s (~400 days) | CSRF protection; must match `x-csrftoken` header |
| `ig_did` | Persistent | Device fingerprint — **different per browser profile** |
| `mid` | Persistent | Machine ID |
| `datr` | Persistent | Browser tracking |
| `rur` | Per-request | Routing hint; refreshed on every response |

**Safe session expiry default**: **7 days**. Conservative vs the 90-day cookie lifetime.
Rationale: IG can revoke sessions server-side at any time for suspicious activity.
A short window limits blast radius if the session is compromised or the account flagged.

**Session import flow:**
1. User logs into burner account in a private browser window over VPN
2. User exports cookies via devtools (curl-as-fetch or cookie export extension)
3. User pastes cookie values into the app UI
4. Backend stores cookies encrypted (AES-256-GCM, per existing secret.key infra)
5. Backend sets `session_expires_at = now() + configured_session_ttl` (default 7 days)

### 9.4 Safety & Observability Requirements

These requirements apply to the `IGClient` implementation stories (S-014, S-015):

#### 9.4.1 Session Expiry Guard

- Configurable `ig_session_ttl` in `config.yaml` (default: `168h` / 7 days)
- On session import, store `session_expires_at` timestamp in SQLite
- Before each API call, check `session_expires_at`
- On expiry: **halt the pull job**, send a Discord notification via webhook:
  `"⚠️ IG session expired for account @{handle}. Re-import cookies to resume."`
- Do not attempt to use an expired session (avoids triggering IG suspicion on stale tokens)

#### 9.4.2 Throttle & Denial Logging

Every IG API call must log to SQLite `ig_request_log` table:

| Column | Type | Description |
|--------|------|-------------|
| `id` | INTEGER PK | Auto-increment |
| `timestamp` | DATETIME | Request time (UTC) |
| `account_id` | INTEGER FK | Linked IG account |
| `endpoint` | TEXT | e.g. `/api/v1/feed/user/{id}/` |
| `http_status` | INTEGER | Response code |
| `is_throttled` | BOOLEAN | `true` if 429 or rate-limit signal |
| `is_denied` | BOOLEAN | `true` if 401/403/checkpoint |
| `response_time_ms` | INTEGER | Latency |
| `public_ip` | TEXT | External IP used for this request (see 9.4.3) |
| `error_detail` | TEXT | Truncated error body (max 500 chars) |

On 429: log, NAK with backoff, send Discord warning.
On 401/403/checkpoint: log, **halt job immediately**, send Discord alert.

#### 9.4.3 Public IP Logging & Rotation Check

- On job start, resolve public IP via `https://api.ipify.org` (or similar)
- Store in `ig_request_log.public_ip` for each request batch
- On subsequent job start, compare current IP to last-used IP for this account
- If IP matches previous session: **warn via Discord**:
  `"⚠️ Same public IP {ip} as last session for @{handle}. Reconnect VPN for IP rotation."`
- User can override/dismiss (not a hard block — just a warning)
- IP check is lightweight (one external call per job start, not per request)

#### 9.4.4 Account Isolation

- Private browser window = separate `ig_did` (confirmed in PoC: different device fingerprint)
- VPN = separate IP from primary browsing
- Burner account = separate `ds_user_id`, email, phone
- **Chain ban vectors mitigated**: no shared device ID, no shared IP, no shared phone/email
- Document this in user-facing setup guide

### 9.5 Revised Integration Architecture

```
┌─────────────────────────────────────────┐
│  Go Backend                             │
│  ┌───────────────┐                      │
│  │ IGClient      │ (interface)          │
│  │ interface      │                      │
│  └───────┬───────┘                      │
│          │                              │
│  ┌───────┴───────┐  ┌────────────────┐  │
│  │ HTTPIGClient  │  │ ig_request_log │  │
│  │               │  │ (SQLite)       │  │
│  │ - net/http    │  └────────────────┘  │
│  │ - cookie jar  │                      │
│  │ - session TTL │  ┌────────────────┐  │
│  │ - IP checker  │  │ Discord notify │  │
│  │ - rate pacer  │  │ (webhook)      │  │
│  └───────┬───────┘  └────────────────┘  │
│          │                              │
│          │ HTTPS (browser-like headers) │
└──────────┼──────────────────────────────┘
           │
    Instagram REST API
    /api/v1/feed/user/{id}/
```

### 9.6 Updated Downstream Story Impact

| Story | Status | Notes |
|-------|--------|-------|
| **S-014** | Unblocked | `IGClient` interface + `HTTPIGClient` implementation. **No instaloader dependency.** |
| **S-015** | Unblocked | Media pull job uses REST feed endpoint. Carousel handling via `carousel_media[]`. |
| **S-016** | Unblocked | No change — works with downloaded files regardless of source. |
| **S-011** | Unblocked | Auth flow is cookie import via UI, not programmatic login. Session TTL + IP tracking. |
| **NEW** | — | Need story for `ig_request_log` schema + Discord notification integration. |

### 9.7 Updated Risks

| Risk | Likelihood | Impact | Mitigation |
|------|-----------|--------|------------|
| REST feed endpoint deprecated | Low | High | `IGClient` interface allows swap; monitor for 404/redirect |
| Session invalidated by IG | Medium | Medium | 7-day default TTL; halt + Discord alert on auth failure; easy re-import |
| IP correlation across sessions | Medium | Medium | Public IP logged; rotation warning on job start |
| Chain ban via shared device ID | Low | High | Confirmed: private window has separate `ig_did`; documented isolation procedure |
| CDN URL format changes | Low | Low | URLs are standard; parse `image_versions2.candidates[]` dynamically |
| 429 during large pulls | High | Medium | Go pacer (configurable interval); log all throttles; backoff on 429 |
