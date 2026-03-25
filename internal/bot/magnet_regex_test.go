package bot

import (
	"testing"
)

// TestMagnetRegex_PackageLevelVar verifies that the package-level magnetRegex variable is compiled
// correctly (not nil) and matches the expected magnet link pattern.
func TestMagnetRegex_PackageLevelVar(t *testing.T) {
	if magnetRegex == nil {
		t.Fatal("magnetRegex package variable is nil")
	}
}

// TestMagnetRegex_Matches tests that the package-level magnetRegex correctly identifies magnet links.
func TestMagnetRegex_Matches(t *testing.T) {
	tests := []struct {
		name    string
		input   string
		wantHit bool
	}{
		{
			name:    "pure magnet link",
			input:   "magnet:?xt=urn:btih:ABCDEF1234567890abcdef&dn=somefile",
			wantHit: true,
		},
		{
			name:    "magnet link with display name and trackers",
			input:   "magnet:?xt=urn:btih:abc123def456abc123def456abc123def456abc1&dn=MyFile.mkv&tr=udp://tracker.example.com:1234",
			wantHit: true,
		},
		{
			name:    "magnet link embedded in surrounding text",
			input:   "Please add this: magnet:?xt=urn:btih:DEADBEEF1234 to the queue",
			wantHit: true,
		},
		{
			name:    "magnet link embedded with newline after hash",
			input:   "magnet:?xt=urn:btih:DEADBEEF1234\nextra text",
			wantHit: true,
		},
		{
			name:    "no magnet link - plain URL",
			input:   "https://example.com/file.torrent",
			wantHit: false,
		},
		{
			name:    "no magnet link - empty string",
			input:   "",
			wantHit: false,
		},
		{
			name:    "no magnet link - partial prefix only",
			input:   "magnet:?xt=urn:btih:",
			wantHit: false, // No hash characters after "btih:"
		},
		{
			name:    "no magnet link - wrong scheme",
			input:   "torrent:?xt=urn:btih:ABC123",
			wantHit: false,
		},
		{
			name:    "magnet link hex-only hash",
			input:   "magnet:?xt=urn:btih:a1b2c3d4e5f6a1b2c3d4e5f6a1b2c3d4e5f6a1b2",
			wantHit: true,
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := magnetRegex.FindString(tt.input) != ""
			if got != tt.wantHit {
				t.Errorf("magnetRegex match on %q: got %v, want %v", tt.input, got, tt.wantHit)
			}
		})
	}
}

// TestMagnetRegex_FindString verifies that FindString extracts the magnet link.
// Note: magnetRegex's pattern is greedy (ends with .*) and will include trailing text.
func TestMagnetRegex_FindString(t *testing.T) {
	input := "Check this out: magnet:?xt=urn:btih:ABC123DEF456 thanks!"
	match := magnetRegex.FindString(input)
	if match == "" {
		t.Fatal("expected a match, got empty string")
	}
	// The extracted match should start with the magnet scheme.
	const wantPrefix = "magnet:?xt=urn:btih:"
	if len(match) < len(wantPrefix) || match[:len(wantPrefix)] != wantPrefix {
		t.Errorf("FindString() = %q, want prefix %q", match, wantPrefix)
	}

	// Verify the full extracted value to ensure we understand the greedy behavior.
	// The regex pattern ends with .* so it captures everything after the hash until end of string.
	const wantExact = "magnet:?xt=urn:btih:ABC123DEF456 thanks!"
	if match != wantExact {
		t.Errorf("FindString() = %q, want exact match %q (regex is greedy and includes trailing text)", match, wantExact)
	}
}

// TestMagnetRegex_RegressionNoPartialBtih ensures the regex does not match an incomplete btih hash.
// This is a boundary/regression test added for extra confidence.
func TestMagnetRegex_RegressionNoPartialBtih(t *testing.T) {
	// "btih:" with nothing after it — should not match because [a-zA-Z0-9]+ requires at least one char.
	input := "magnet:?xt=urn:btih:"
	if magnetRegex.MatchString(input) {
		t.Errorf("magnetRegex should not match a magnet link with empty hash, but it did for input %q", input)
	}
}
