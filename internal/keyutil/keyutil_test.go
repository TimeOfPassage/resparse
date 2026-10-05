package keyutil

import "testing"

func TestNormalizeKey(t *testing.T) {
	cases := map[string]string{
		"  videos/demo.mp4 ": "videos/demo.mp4",
		"///a/b":             "a/b",
		"":                   "",
		"   ":                "",
	}
	for in, want := range cases {
		if got := NormalizeKey(in); got != want {
			t.Errorf("NormalizeKey(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestExtensionOf(t *testing.T) {
	cases := map[string]string{
		"videos/Demo.MP4": ".mp4",
		"README.md":       ".md",
		"a/b/c.mkv":       ".mkv",
		"noext":           "",
		".gitignore":      "",
		"archive.tar.gz":  ".gz",
		"weird.abcdefghi": "",
	}
	for in, want := range cases {
		if got := ExtensionOf(in); got != want {
			t.Errorf("ExtensionOf(%q) = %q, want %q", in, got, want)
		}
	}
}
