package model

import (
	"regexp"
	"strings"
	"unicode"

	"golang.org/x/text/unicode/norm"
)

// MaxSlugLength is the maximum allowed length for a slug.
const MaxSlugLength = 64

var (
	nonAlphanumHyphen = regexp.MustCompile(`[^a-z0-9-]+`)
	multiHyphen       = regexp.MustCompile(`-{2,}`)
)

// GenerateSlug creates a URL-friendly slug from the given name.
// Rules: lowercase, hyphens for separators, no special chars, max 64 chars.
// Unicode letters are transliterated to ASCII where possible (via NFKD
// decomposition), otherwise stripped.
func GenerateSlug(name string) string {
	// NFKD decomposition to separate base letters from combining marks.
	decomposed := norm.NFKD.String(name)

	// Keep only ASCII letters, digits, spaces, and hyphens.
	var b strings.Builder
	for _, r := range decomposed {
		if r <= 127 && (unicode.IsLetter(r) || unicode.IsDigit(r) || r == ' ' || r == '-' || r == '_') {
			b.WriteRune(r)
		}
	}

	s := strings.ToLower(b.String())
	// Replace spaces and underscores with hyphens.
	s = strings.ReplaceAll(s, " ", "-")
	s = strings.ReplaceAll(s, "_", "-")
	// Collapse multiple hyphens.
	s = multiHyphen.ReplaceAllString(s, "-")
	// Remove non-alphanumeric/hyphen chars (safety net).
	s = nonAlphanumHyphen.ReplaceAllString(s, "")
	// Trim leading/trailing hyphens.
	s = strings.Trim(s, "-")

	// Truncate to max length, avoiding mid-word truncation at hyphen boundary.
	if len(s) > MaxSlugLength {
		s = s[:MaxSlugLength]
		s = strings.TrimRight(s, "-")
	}

	return s
}
