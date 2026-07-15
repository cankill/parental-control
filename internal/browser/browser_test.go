package browser

import "testing"

func TestDomain(t *testing.T) {
	cases := map[string]string{
		"https://www.youtube.com/watch?v=abc": "youtube.com",
		"https://github.com/user/repo":        "github.com",
		"http://example.org":                  "example.org",
		"https://sub.domain.co/path":          "sub.domain.co",
		"":                                    "",
		"not a url":                           "",
		"about:blank":                         "",
	}
	for in, want := range cases {
		if got := Domain(in); got != want {
			t.Errorf("Domain(%q) = %q, want %q", in, got, want)
		}
	}
}

func TestIsBrowser(t *testing.T) {
	if !IsBrowser("com.google.Chrome") {
		t.Error("Chrome should be a browser")
	}
	if IsBrowser("com.apple.Terminal") {
		t.Error("Terminal should not be a browser")
	}
}
