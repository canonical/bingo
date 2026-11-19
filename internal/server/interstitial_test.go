package server_test

import (
	"context"
	"io"
	"net/http"
	"net/http/httptest"
	"os"
	"path/filepath"
	"strconv"
	"sync/atomic"
	"testing"
	"time"

	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/require"

	"bingo/internal/auth"
	"bingo/internal/config"
	"bingo/internal/paste"
	"bingo/internal/server"
)

const interstitialMarker = "This pastebin is moving"

// newLegacyServer starts bingo with the PS5 interstitial enabled. With
// withAuth it also enables OIDC (against a fake IdP) and returns a valid
// session cookie for "signed in" requests.
func newLegacyServer(t *testing.T, repo paste.Repository, withAuth bool) (*httptest.Server, *http.Cookie) {
	t.Helper()
	cfg := &config.Config{
		BaseURL:           "https://example.com",
		MaxPasteSizeBytes: 5 * 1024 * 1024,
		WebDir:            t.TempDir(),
		LegacyBaseURL:     "https://legacy.example.com",
	}
	require.NoError(t, os.WriteFile(filepath.Join(cfg.WebDir, "index.html"),
		[]byte("<html><head></head><body>bingo-spa</body></html>"), 0o644))

	var provider *auth.Provider
	var userRepo *auth.UserRepository
	var session *http.Cookie
	if withAuth {
		idp := newFakeOIDCServer(t)
		cfg.OIDCIssuerURL = idp.URL
		cfg.OIDCClientID = "test-client"
		cfg.OIDCClientSecret = "test-secret"
		cfg.OIDCRedirectURL = "https://example.com/auth/callback"
		cfg.SessionSecret = "test-session-secret"
		var err error
		provider, err = auth.NewProvider(t.Context(), cfg)
		require.NoError(t, err)
		userRepo = new(auth.UserRepository)

		rec := httptest.NewRecorder()
		require.NoError(t, auth.NewCodec(cfg.SessionSecret).Set(rec,
			&auth.Session{UserID: 1, Sub: "s", Email: "u@example.com"}))
		session = rec.Result().Cookies()[0]
	}

	ts := httptest.NewServer(server.New(cfg, nil, repo, provider, userRepo))
	t.Cleanup(ts.Close)
	return ts, session
}

func getWithCookies(t *testing.T, url string, cookies ...*http.Cookie) (*http.Response, string) {
	t.Helper()
	req, err := http.NewRequest(http.MethodGet, url, nil)
	require.NoError(t, err)
	for _, c := range cookies {
		req.AddCookie(c)
	}
	resp, err := http.DefaultClient.Do(req)
	require.NoError(t, err)
	defer resp.Body.Close()
	body, err := io.ReadAll(resp.Body)
	require.NoError(t, err)
	return resp, string(body)
}

// The page is served under a CSP with no 'unsafe-inline' for scripts, so it
// must not depend on inline JS (the footer year used to be filled in that way).
func TestInterstitial_noInlineScript_yearRenderedServerSide(t *testing.T) {
	ts, _ := newLegacyServer(t, defaultRepo(), false)

	_, body := getWithCookies(t, ts.URL+"/")

	assert.NotContains(t, body, "<script")
	assert.Contains(t, body, "&copy; "+strconv.Itoa(time.Now().Year()))
}

func TestInterstitial_noDecommissionDate_noPlaceholder(t *testing.T) {
	ts, _ := newLegacyServer(t, defaultRepo(), false)

	_, body := getWithCookies(t, ts.URL+"/")

	assert.Contains(t, body, interstitialMarker)
	assert.NotContains(t, body, "[DECOMMISSION DATE]")
}

// "/" returns different pages depending on cookies and ?continue, so none of
// these responses may be cached.
func TestInterstitial_responsesNotCacheable(t *testing.T) {
	ts, _ := newLegacyServer(t, defaultRepo(), false)

	for _, path := range []string{"/", "/?continue=1", "/AbCdEfGhIj"} {
		resp, _ := getWithCookies(t, ts.URL+path)
		assert.Equal(t, "no-store", resp.Header.Get("Cache-Control"), path)
	}
}

func TestInterstitial_continueChoiceIsRemembered(t *testing.T) {
	ts, _ := newLegacyServer(t, defaultRepo(), false)

	resp, body := getWithCookies(t, ts.URL+"/?continue=1")
	require.Contains(t, body, "bingo-spa")
	var dismissed *http.Cookie
	for _, c := range resp.Cookies() {
		if c.Name == "ps5_notice_dismissed" {
			dismissed = c
		}
	}
	require.NotNil(t, dismissed, "choosing bingo should set a cookie")
	assert.Equal(t, http.SameSiteLaxMode, dismissed.SameSite, "must survive links from other sites")

	_, body = getWithCookies(t, ts.URL+"/", dismissed)
	assert.Contains(t, body, "bingo-spa")
	assert.NotContains(t, body, interstitialMarker)

	// Unknown keys still get the notice: that's how old PS5 links reach the
	// legacy instance.
	_, body = getWithCookies(t, ts.URL+"/AbCdEfGhIj", dismissed)
	assert.Contains(t, body, interstitialMarker)
}

// With auth on, signed-out users get 401 from /api/v1/pastes/{key}. The page
// route must not reveal whether a key exists, or touch the repo, for them.
func TestInterstitial_authEnabled_signedOutCannotProbePasteKeys(t *testing.T) {
	var lookups atomic.Int32
	repo := defaultRepo()
	repo.getByKeyFn = func(_ context.Context, key string) (*paste.Paste, error) {
		lookups.Add(1)
		if key == "RealPaste1" {
			return &paste.Paste{Key: key, ExpiresAt: time.Now().Add(time.Hour)}, nil
		}
		return nil, paste.ErrNotFound
	}
	ts, session := newLegacyServer(t, repo, true)

	_, existing := getWithCookies(t, ts.URL+"/RealPaste1")
	_, missing := getWithCookies(t, ts.URL+"/NoSuchKey1")
	assert.Contains(t, existing, interstitialMarker)
	assert.Contains(t, missing, interstitialMarker)
	assert.Zero(t, lookups.Load(), "signed-out requests must not hit the repository")

	// Signed in, lookups happen and an existing paste still opens in bingo.
	_, body := getWithCookies(t, ts.URL+"/RealPaste1", session)
	assert.Contains(t, body, "bingo-spa")
	_, body = getWithCookies(t, ts.URL+"/NoSuchKey1", session)
	assert.Contains(t, body, interstitialMarker)
}

// r.URL.Path is already decoded, so the legacy link must re-escape it:
// otherwise %3F / %23 in an old link become a real query / fragment.
func TestInterstitial_legacyLinkKeepsPathEncoding(t *testing.T) {
	ts, _ := newLegacyServer(t, defaultRepo(), false)

	_, body := getWithCookies(t, ts.URL+"/abc%3Fraw%3D1")
	assert.Contains(t, body, `href="https://legacy.example.com/abc%3Fraw=1"`)

	_, body = getWithCookies(t, ts.URL+"/abc%23frag")
	assert.Contains(t, body, `href="https://legacy.example.com/abc%23frag"`)
}

// Non-GET methods shouldn't trigger a paste lookup or get a 200 HTML page.
func TestServeStaticFiles_nonGETMethodsNotAllowed(t *testing.T) {
	var lookups atomic.Int32
	repo := defaultRepo()
	repo.getByKeyFn = func(context.Context, string) (*paste.Paste, error) {
		lookups.Add(1)
		return nil, paste.ErrNotFound
	}
	ts, _ := newLegacyServer(t, repo, false)

	for _, method := range []string{http.MethodPost, http.MethodPut, http.MethodDelete} {
		req, err := http.NewRequest(method, ts.URL+"/AbCdEfGhIj", nil)
		require.NoError(t, err)
		resp, err := http.DefaultClient.Do(req)
		require.NoError(t, err)
		resp.Body.Close()
		assert.Equal(t, http.StatusMethodNotAllowed, resp.StatusCode, method)
	}
	assert.Zero(t, lookups.Load(), "non-GET requests must not hit the repository")

	resp, err := http.Head(ts.URL + "/AbCdEfGhIj")
	require.NoError(t, err)
	resp.Body.Close()
	assert.Equal(t, http.StatusOK, resp.StatusCode, "HEAD still works")
}

// Legacy PS5 links like /p/<id>/ never match a bingo route: they must reach
// the legacy instance with the trailing slash intact, for any user.
func TestInterstitial_legacyPasteLinksPreservePath(t *testing.T) {
	ts, session := newLegacyServer(t, defaultRepo(), true)

	for path, want := range map[string]string{
		"/p/V3bRMMcPcC/":       "https://legacy.example.com/p/V3bRMMcPcC/",
		"/p/V3bRMMcPcC/plain/": "https://legacy.example.com/p/V3bRMMcPcC/plain/",
		"/=V3bRMMcPcC/plain/":  "https://legacy.example.com/=V3bRMMcPcC/plain/",
		"/123/plain/":          "https://legacy.example.com/123/plain/",
		"/AbCdEfGhIj/":         "https://legacy.example.com/AbCdEfGhIj/",
	} {
		for _, cookies := range [][]*http.Cookie{nil, {session}} {
			_, body := getWithCookies(t, ts.URL+path, cookies...)
			assert.Contains(t, body, interstitialMarker, path)
			assert.Contains(t, body, `href="`+want+`"`, path)
		}
	}
}
