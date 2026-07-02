// Package paste defines paste domain types, the repository interface, and the language registry.
package paste

import (
	"context"
	"errors"
	"fmt"
	"sort"
	"time"
)

// ErrNotFound is returned when a paste key does not exist or has expired.
var ErrNotFound = errors.New("paste not found")

// Paste is the full paste entity as stored in the database.
type Paste struct {
	ID        int64
	Key       string
	Content   string
	Language  string
	Title     string
	SizeBytes int
	ExpiresAt time.Time
	CreatedAt time.Time
	OwnerID   *int64
}

// CreateParams holds the caller-supplied fields for creating a new paste.
type CreateParams struct {
	Content   string
	Language  string
	Title     string
	ExpiresIn ExpiresIn
	OwnerID   *int64 // nil for anonymous pastes
}

// Repository defines the storage operations for pastes.
// Implementations must be safe for concurrent use.
type Repository interface {
	// Create persists a new paste, generating a collision-resistant key internally.
	Create(ctx context.Context, params CreateParams) (*Paste, error)
	// GetByKey retrieves a paste by its short key. Returns ErrNotFound when absent.
	GetByKey(ctx context.Context, key string) (*Paste, error)
	// Delete removes a paste by key. A missing key is not an error.
	Delete(ctx context.Context, key string) error
	// DeleteExpired removes all pastes whose expires_at is in the past.
	// Returns the number of rows deleted.
	DeleteExpired(ctx context.Context) (int64, error)
	// ListByOwner returns up to limit active (non-expired) pastes for ownerID,
	// ordered by created_at descending.
	ListByOwner(ctx context.Context, ownerID int64, limit int) ([]*Paste, error)
}

// ExpiresIn is the set of allowed paste expiration durations.
type ExpiresIn string

const (
	ExpiresIn1d  ExpiresIn = "1d"
	ExpiresIn1w  ExpiresIn = "1w"
	ExpiresIn1mo ExpiresIn = "1mo"
	ExpiresIn3mo ExpiresIn = "3mo"
	ExpiresIn1y  ExpiresIn = "1y"
)

var expiryDurations = map[ExpiresIn]time.Duration{
	ExpiresIn1d:  24 * time.Hour,
	ExpiresIn1w:  7 * 24 * time.Hour,
	ExpiresIn1mo: 30 * 24 * time.Hour,
	ExpiresIn3mo: 90 * 24 * time.Hour,
	ExpiresIn1y:  365 * 24 * time.Hour,
}

// ParseExpiresIn validates and returns an ExpiresIn from a raw string.
// Valid values: "1d", "1w", "1mo", "3mo", "1y".
func ParseExpiresIn(s string) (ExpiresIn, error) {
	e := ExpiresIn(s)
	if _, ok := expiryDurations[e]; !ok {
		return "", fmt.Errorf("invalid expires_in %q: must be one of 1d, 1w, 1mo, 3mo, 1y", s)
	}
	return e, nil
}

// Duration returns the time.Duration for this ExpiresIn value.
func (e ExpiresIn) Duration() time.Duration {
	return expiryDurations[e]
}

// validLanguages is the set of accepted language identifiers.
// Keys match react-syntax-highlighter (Prism) language names used by the frontend.
var validLanguages = map[string]struct{}{
	"plaintext":               {},
	"abap":                     {},
	"abnf":                     {},
	"actionscript":             {},
	"ada":                      {},
	"agda":                     {},
	"al":                       {},
	"antlr4":                   {},
	"apacheconf":               {},
	"apex":                     {},
	"apl":                      {},
	"applescript":              {},
	"aql":                      {},
	"arduino":                  {},
	"arff":                     {},
	"armasm":                   {},
	"arturo":                   {},
	"asciidoc":                 {},
	"asm6502":                  {},
	"asmatmel":                 {},
	"aspnet":                   {},
	"autohotkey":               {},
	"autoit":                   {},
	"avisynth":                 {},
	"avro-idl":                 {},
	"awk":                      {},
	"bash":                     {},
	"basic":                    {},
	"batch":                    {},
	"bbcode":                   {},
	"bbj":                      {},
	"bicep":                    {},
	"birb":                     {},
	"bison":                    {},
	"bnf":                      {},
	"bqn":                      {},
	"brainfuck":                {},
	"brightscript":             {},
	"bro":                      {},
	"bsl":                      {},
	"c":                        {},
	"cfscript":                 {},
	"chaiscript":               {},
	"cil":                      {},
	"cilkc":                    {},
	"cilkcpp":                  {},
	"clojure":                  {},
	"cmake":                    {},
	"cobol":                    {},
	"coffeescript":             {},
	"concurnas":                {},
	"cooklang":                 {},
	"coq":                      {},
	"cpp":                      {},
	"crystal":                  {},
	"csharp":                   {},
	"cshtml":                   {},
	"csp":                      {},
	"css":                      {},
	"csv":                      {},
	"cue":                      {},
	"cypher":                   {},
	"d":                        {},
	"dart":                     {},
	"dataweave":                {},
	"dax":                      {},
	"dhall":                    {},
	"diff":                     {},
	"django":                   {},
	"dns-zone-file":            {},
	"docker":                   {},
	"dot":                      {},
	"ebnf":                     {},
	"editorconfig":             {},
	"eiffel":                   {},
	"ejs":                      {},
	"elixir":                   {},
	"elm":                      {},
	"erb":                      {},
	"erlang":                   {},
	"etlua":                    {},
	"excel-formula":            {},
	"factor":                   {},
	"false":                    {},
	"firestore-security-rules": {},
	"flow":                     {},
	"fortran":                  {},
	"fsharp":                   {},
	"ftl":                      {},
	"gap":                      {},
	"gcode":                    {},
	"gdscript":                 {},
	"gedcom":                   {},
	"gettext":                  {},
	"gherkin":                  {},
	"git":                      {},
	"glsl":                     {},
	"gml":                      {},
	"gn":                       {},
	"go":                       {},
	"go-module":                {},
	"gradle":                   {},
	"graphql":                  {},
	"groovy":                   {},
	"haml":                     {},
	"handlebars":               {},
	"haskell":                  {},
	"haxe":                     {},
	"hcl":                      {},
	"hlsl":                     {},
	"hoon":                     {},
	"hpkp":                     {},
	"hsts":                     {},
	"http":                     {},
	"ichigojam":                {},
	"icon":                     {},
	"icu-message-format":       {},
	"idris":                    {},
	"iecst":                    {},
	"ignore":                   {},
	"inform7":                  {},
	"ini":                      {},
	"io":                       {},
	"j":                        {},
	"java":                     {},
	"javadoc":                  {},
	"javascript":               {},
	"javastacktrace":           {},
	"jexl":                     {},
	"jolie":                    {},
	"jq":                       {},
	"jsdoc":                    {},
	"json":                     {},
	"json5":                    {},
	"jsonp":                    {},
	"jsstacktrace":             {},
	"jsx":                      {},
	"julia":                    {},
	"keepalived":               {},
	"keyman":                   {},
	"kotlin":                   {},
	"kumir":                    {},
	"kusto":                    {},
	"latex":                    {},
	"latte":                    {},
	"less":                     {},
	"lilypond":                 {},
	"linker-script":            {},
	"liquid":                   {},
	"lisp":                     {},
	"livescript":               {},
	"llvm":                     {},
	"log":                      {},
	"lolcode":                  {},
	"lua":                      {},
	"magma":                    {},
	"makefile":                 {},
	"markdown":                 {},
	"markup":                   {},
	"mata":                     {},
	"matlab":                   {},
	"maxscript":                {},
	"mel":                      {},
	"mermaid":                  {},
	"metafont":                 {},
	"mizar":                    {},
	"mongodb":                  {},
	"monkey":                   {},
	"moonscript":               {},
	"n1ql":                     {},
	"n4js":                     {},
	"nand2tetris-hdl":          {},
	"naniscript":               {},
	"nasm":                     {},
	"neon":                     {},
	"nevod":                    {},
	"nginx":                    {},
	"nim":                      {},
	"nix":                      {},
	"nsis":                     {},
	"objectivec":               {},
	"ocaml":                    {},
	"odin":                     {},
	"opencl":                   {},
	"openqasm":                 {},
	"oz":                       {},
	"parigp":                   {},
	"parser":                   {},
	"pascal":                   {},
	"pascaligo":                {},
	"pcaxis":                   {},
	"peoplecode":               {},
	"perl":                     {},
	"php":                      {},
	"phpdoc":                   {},
	"plant-uml":                {},
	"plsql":                    {},
	"powerquery":               {},
	"powershell":               {},
	"processing":               {},
	"prolog":                   {},
	"promql":                   {},
	"properties":               {},
	"protobuf":                 {},
	"psl":                      {},
	"pug":                      {},
	"puppet":                   {},
	"pure":                     {},
	"purebasic":                {},
	"purescript":               {},
	"python":                   {},
	"q":                        {},
	"qml":                      {},
	"qore":                     {},
	"qsharp":                   {},
	"r":                        {},
	"racket":                   {},
	"reason":                   {},
	"regex":                    {},
	"rego":                     {},
	"renpy":                    {},
	"rescript":                 {},
	"rest":                     {},
	"rip":                      {},
	"roboconf":                 {},
	"robotframework":           {},
	"ruby":                     {},
	"rust":                     {},
	"sas":                      {},
	"sass":                     {},
	"scala":                    {},
	"scheme":                   {},
	"scss":                     {},
	"shell-session":            {},
	"smali":                    {},
	"smalltalk":                {},
	"smarty":                   {},
	"sml":                      {},
	"solidity":                 {},
	"solution-file":            {},
	"soy":                      {},
	"sparql":                   {},
	"splunk-spl":               {},
	"sqf":                      {},
	"sql":                      {},
	"squirrel":                 {},
	"stan":                     {},
	"stata":                    {},
	"stylus":                   {},
	"supercollider":            {},
	"swift":                    {},
	"systemd":                  {},
	"t4-cs":                    {},
	"t4-templating":            {},
	"t4-vb":                    {},
	"tap":                      {},
	"tcl":                      {},
	"textile":                  {},
	"toml":                     {},
	"tremor":                   {},
	"tsx":                      {},
	"tt2":                      {},
	"turtle":                   {},
	"twig":                     {},
	"typescript":               {},
	"typoscript":               {},
	"unrealscript":             {},
	"uorazor":                  {},
	"uri":                      {},
	"v":                        {},
	"vala":                     {},
	"vbnet":                    {},
	"velocity":                 {},
	"verilog":                  {},
	"vhdl":                     {},
	"vim":                      {},
	"visual-basic":             {},
	"warpscript":               {},
	"wasm":                     {},
	"web-idl":                  {},
	"wgsl":                     {},
	"wiki":                     {},
	"wolfram":                  {},
	"wren":                     {},
	"xeora":                    {},
	"xml-doc":                  {},
	"xojo":                     {},
	"xquery":                   {},
	"yaml":                     {},
	"yang":                     {},
	"zig":                      {},
}

// IsValidLanguage reports whether lang is in the supported language registry.
func IsValidLanguage(lang string) bool {
	_, ok := validLanguages[lang]
	return ok
}

// AllLanguages returns a sorted slice of all supported language identifiers.
func AllLanguages() []string {
	langs := make([]string, 0, len(validLanguages))
	for l := range validLanguages {
		langs = append(langs, l)
	}
	sort.Strings(langs)
	return langs
}
