package recettescabas

import (
	"regexp"
	"strings"
)

var accents = strings.NewReplacer(
	"à", "a", "â", "a", "ä", "a",
	"é", "e", "è", "e", "ê", "e", "ë", "e",
	"î", "i", "ï", "i",
	"ô", "o", "ö", "o",
	"ù", "u", "û", "u", "ü", "u",
	"ÿ", "y", "ç", "c", "œ", "oe", "æ", "ae",
	" ", " ", "’", "'",
)

func normalize(s string) string {
	return accents.Replace(strings.ToLower(s))
}

// match returns the first word of text matching one of keywords, or "" if none does.
// A keyword matches a whole word, optionally followed by a plural "s" or "x",
// ignoring case and accents. The returned word is normalized.
func match(text string, keywords []string) string {
	text = normalize(text)

	for _, kw := range keywords {
		kw = strings.TrimSpace(normalize(kw))
		if kw == "" {
			continue
		}

		re := regexp.MustCompile(`\b` + regexp.QuoteMeta(kw) + `[sx]?\b`)
		if found := re.FindString(text); found != "" {
			return found
		}
	}

	return ""
}
