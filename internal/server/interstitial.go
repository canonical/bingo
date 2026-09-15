package server

import (
	"embed"
	"errors"
	"html/template"
	"log/slog"
	"net/http"
	"strings"

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
}

// continueQueryParam is the query parameter used on "/" to bypass the
// migration interstitial and reach bingo's real home page, e.g. after a
// user clicks "Go to bingo" on the interstitial itself.
const continueQueryParam = "continue"

// serveInterstitial renders the PS5-decommission migration interstitial,
// offering the user a choice between bingo and the legacy PS5 instance at
// the equivalent path. path is the cleaned request path (e.g. "/" or
// "/AbCdEfGhIj") as computed by serveStaticFiles.
func (s *Server) serveInterstitial(w http.ResponseWriter, path string) {
	data := interstitialData{
		LegacyURL:        strings.TrimSuffix(s.cfg.LegacyBaseURL, "/") + path,
		ContinueURL:      s.cfg.BasePath() + "/?" + continueQueryParam + "=1",
		DecommissionDate: s.cfg.PS5DecommissionDate,
	}
	w.Header().Set("Content-Type", "text/html; charset=utf-8")
	if err := interstitialTemplate.Execute(w, data); err != nil {
		slog.Error("render ps5 migration interstitial", "err", err)
		http.Error(w, "Internal Server Error", http.StatusInternalServerError)
	}
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
