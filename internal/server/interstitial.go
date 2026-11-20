package server

import (
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"net/url"
	"regexp"
	"strings"
	"time"

	"bingo/internal/auth"
	"bingo/internal/paste"
)

//go:embed templates/interstitial.html.tmpl
var interstitialFS embed.FS

var interstitialTemplate = template.Must(template.ParseFS(interstitialFS, "templates/interstitial.html.tmpl"))

// interstitialData is the template context for the PS5 migration interstitial.
type interstitialData struct {
	// LegacyURL is the equivalent path on the legacy PS5 instance
	// (cfg.LegacyBaseURL + the current request path), so the "Continue to
	// legacy pastebin" action preserves paste-specific links (e.g. a user
	// following an old /AbCdEfGhIj link lands on that same paste there)
	// rather than always sending them to the legacy homepage.
	LegacyURL string
	// ContinueURL takes the user to bingo's real home page, bypassing the
	// interstitial via the continueQueryParam marker (see serveStaticFiles).
	ContinueURL string
	// DecommissionDate is an optional human-readable decommission
	// date/timeline, sourced from cfg.PS5DecommissionDate.
	DecommissionDate string
	// Year is rendered in the footer. Done server-side because the CSP set
	// by securityHeadersMiddleware blocks inline scripts.
	Year int
}

// continueQueryParam is the query parameter used on "/" to bypass the
// migration interstitial and reach bingo's real home page, e.g. after a
// user clicks "Go to bingo" on the interstitial itself.
const continueQueryParam = "continue"

// noticeDismissedCookie remembers that the user chose bingo on the
// interstitial, so a later visit to "/" (bookmark, link from chat) goes
// straight to bingo instead of showing the notice again.
const noticeDismissedCookie = "ps5_notice_dismissed"

// skipInterstitialAtRoot reports whether a request for "/" should go straight
// to bingo's UI: the user chose bingo now (continue marker) or earlier
// (cookie), or is signed in. The continue marker also sets the cookie so the
// choice sticks.
func (s *Server) skipInterstitialAtRoot(w http.ResponseWriter, r *http.Request) bool {
	if r.URL.Query().Get(continueQueryParam) != "" {
		http.SetCookie(w, &http.Cookie{
			Name:     noticeDismissedCookie,
			Value:    "1",
			Path:     "/",
			MaxAge:   30 * 24 * 60 * 60, // 30 days
			HttpOnly: true,
			Secure:   true,
			// Lax, not Strict: it must still be sent when the user arrives
			// from another site (e.g. a link in Mattermost).
			SameSite: http.SameSiteLaxMode,
		})
		return true
	}
	if _, err := r.Cookie(noticeDismissedCookie); err == nil {
		return true
	}
	_, signedIn := auth.FromContext(r.Context())
	return signedIn
}

// canReadPastes mirrors requireAuthMiddleware's gate on /api/: true when auth
// is disabled or the request has a valid session.
func (s *Server) canReadPastes(r *http.Request) bool {
	if s.auth == nil {
		return true
	}
	_, ok := auth.FromContext(r.Context())
	return ok
}

// isReadMethod reports whether the request is one the page routes serve.
func isReadMethod(r *http.Request) bool {
	return r.Method == http.MethodGet || r.Method == http.MethodHead
}

// serveInterstitial renders the PS5-decommission migration interstitial,
// offering the user a choice between bingo and the legacy PS5 instance at
// the equivalent path. path is the request path (e.g. "/" or
// "/p/AbCdEfGhIj/"); its trailing slash is kept, as legacy URLs need it.
func (s *Server) serveInterstitial(w http.ResponseWriter, path string) {
	// path has already been percent-decoded, so re-escape it: otherwise a
	// %3F or %23 in an old link becomes a real query or fragment here.
	data := interstitialData{
		LegacyURL:        strings.TrimSuffix(s.cfg.LegacyBaseURL, "/") + (&url.URL{Path: path}).EscapedPath(),
		ContinueURL:      s.cfg.BasePath() + "/?" + continueQueryParam + "=1",
		DecommissionDate: s.cfg.PS5DecommissionDate,
		Year:             time.Now().Year(),
	}
	w.Header().Set("Cache-Control", "no-store")
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := interstitialTemplate.Execute(w, data); err != nil {
		slog.Error("render ps5 migration interstitial", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
}

// legacyOnlyPath matches legacy PS5 paste URLs that have no bingo equivalent:
// /p/<id>/, and the plain/hide/show sub-pages of /<id>/ and /=<id>/.
var legacyOnlyPath = regexp.MustCompile(`^/(p/[A-Za-z0-9]+(/(plain|hide|show))?|(=[A-Za-z0-9]+|[0-9]+)/(plain|hide|show))/?$`)

// isLegacyOnlyPath reports whether path can only be a legacy PS5 paste link.
// clean has no trailing slash (filepath.Clean), so the optional one is moot.
func isLegacyOnlyPath(path string) bool {
	return legacyOnlyPath.MatchString(path)
}

// isCandidatePasteKey reports whether clean looks like a bingo paste-key
// route (a single path segment, e.g. "/AbCdEfGhIj") as opposed to one of
// the app's other known client-side routes or a nested/unexpected path.
// Used to decide whether an unmatched path is worth a paste-existence
// check before falling back to the migration interstitial.
func isCandidatePasteKey(clean string) bool {
	trimmed := strings.TrimPrefix(clean, "/")
	if trimmed == "" || strings.Contains(trimmed, "/") {
		return false
	}
	switch trimmed {
	case "my-pastes", "error":
		return false
	}
	return true
}

// isPasteNotFound reports whether err indicates the given key does not
// exist as a bingo paste (as opposed to some other repository failure,
// which should not trigger the migration interstitial).
func isPasteNotFound(err error) bool {
	return errors.Is(err, paste.ErrNotFound)
}
