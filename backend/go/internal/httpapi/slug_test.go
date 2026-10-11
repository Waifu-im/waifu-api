package httpapi

import "testing"

// These expectations were computed by applying StringExtensions.ToSlug's steps
// by hand to the C# source, not by copying the Go implementation. A slug is
// part of the public API (/tags/by-slug/{slug}, ?includedTags=) and stored
// image tag rows were written under this rule, so drifting from it makes
// existing rows unaddressable rather than merely producing a different-looking
// URL.
//
// The three cases that a "collapse runs of junk into a dash" shortcut gets
// wrong are the reason this table exists.
func TestSlugifyMatchesDotNet(t *testing.T) {
	cases := []struct {
		in   string
		want string
		why  string
	}{
		{"Waifu Girl", "waifu-girl", "basic"},
		{"Marin Kitagawa", "marin-kitagawa", "two words"},

		// Punctuation is deleted outright, not converted to a separator, because
		// ToSlug removes it in a step that runs BEFORE whitespace becomes a dash.
		{"waifu!!!girl", "waifugirl", "punctuation is deleted, not a separator"},
		{"waifu...girl", "waifugirl", "same for dots"},

		// Existing hyphens survive untouched; only whitespace runs collapse.
		{"wi--fi", "wi--fi", "hyphen runs are preserved"},
		{"waifu -- girl", "waifu----girl", "hyphens and spaces each collapse separately"},

		// Accents are stripped by decomposing, so the first letter is not lost.
		{"Éclair", "eclair", "accented first character keeps its letter"},
		{"Café", "cafe", "accent stripped"},
		{"Pokémon", "pokemon", "accent stripped mid-word"},
		{"Ünïcodé Nâmé", "unicode-name", "every accent stripped"},

		{"  spaced  ", "spaced", "surrounding whitespace trimmed"},
		{"", "", "empty"},
		{"   ", "", "whitespace only"},
		{"!!!", "", "punctuation only"},

		{"Waifu  Girl", "waifu-girl", "a whitespace run collapses to one dash"},
		{"a1", "a1", "digits kept"},
		{"日本語", "", "non-latin script has no ASCII slug form"},
	}

	for _, c := range cases {
		if got := slugify(c.in); got != c.want {
			t.Errorf("slugify(%q) = %q, want %q (%s)", c.in, got, c.want, c.why)
		}
	}
}
