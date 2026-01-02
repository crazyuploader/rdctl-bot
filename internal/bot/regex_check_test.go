package bot

import (
	"regexp"
	"strings"
	"testing"
)

func TestRegexMatching(t *testing.T) {
	// Simulate the supportedRegex slice in Bot
	var supportedRegex []*regexp.Regexp

	// Sample regexes (similar to what RD might return)
	patterns := []string{
		`^https?://(www\.)?rapidgator\.net/.*`,
		// Real API sample with slashes and escaped slashes
		`/(http|https):\/\/(\w+\.)?1fichier\.com\/?.*/`,
	}

	for _, p := range patterns {
		// Apply cleanup logic (mirroring bot.go)
		if len(p) > 2 && p[0] == '/' && p[len(p)-1] == '/' {
			p = p[1 : len(p)-1]
		}
		p = strings.ReplaceAll(p, `\/`, `/`)

		re, err := regexp.Compile(p)
		if err != nil {
			t.Fatalf("Failed to compile pattern %s: %v", p, err)
		}
		supportedRegex = append(supportedRegex, re)
	}

	tests := []struct {
		link    string
		matches bool
	}{
		{"https://rapidgator.net/file/12345/filename.rar", true},
		{"http://www.uploaded.net/file/abcde", false}, // Not in patterns list
		{"https://1fichier.com/?abcdef123", true},
		{"https://google.com/search?q=file", false},
		{"https://example.com/file.rar", false},
	}

	for _, tt := range tests {
		matched := false
		for _, regex := range supportedRegex {
			if regex.MatchString(tt.link) {
				matched = true
				break
			}
		}

		if matched != tt.matches {
			t.Errorf("Link %s: expected match=%v, got match=%v", tt.link, tt.matches, matched)
		}
	}
}
