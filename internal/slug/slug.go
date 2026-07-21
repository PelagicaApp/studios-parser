package slug

import (
	"regexp"
	"strings"

	"golang.org/x/text/unicode/norm"
)

var nonAlphanumericPattern = regexp.MustCompile(`[^a-z0-9]+`)
var edgeDashPattern = regexp.MustCompile(`^-+|-+$`)

func Slugify(value string) string {
	var withoutDiacritics strings.Builder
	for _, r := range norm.NFKD.String(value) {
		if r >= 0x0300 && r <= 0x036f {
			continue
		}
		withoutDiacritics.WriteRune(r)
	}

	result := strings.ToLower(strings.TrimSpace(withoutDiacritics.String()))
	result = nonAlphanumericPattern.ReplaceAllString(result, "-")
	result = edgeDashPattern.ReplaceAllString(result, "")
	return result
}
