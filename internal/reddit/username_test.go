package reddit

import "testing"

func TestNormalizeUsername(t *testing.T) {
	t.Parallel()
	tests := map[string]string{
		"spez":                              "spez",
		" u/spez ":                          "spez",
		"/u/spez/":                          "spez",
		"@spez":                             "spez",
		"https://www.reddit.com/user/spez/": "spez",
		"https://reddit.com/u/spez":         "spez",
	}
	for input, want := range tests {
		input, want := input, want
		t.Run(input, func(t *testing.T) {
			t.Parallel()
			got, err := NormalizeUsername(input)
			if err != nil {
				t.Fatalf("NormalizeUsername(%q): %v", input, err)
			}
			if got != want {
				t.Fatalf("got %q, want %q", got, want)
			}
		})
	}
}

func TestNormalizeUsernameRejectsInvalidInput(t *testing.T) {
	t.Parallel()
	for _, input := range []string{"", "not a name", "https://example.com/u/spez", "https://evilreddit.com/u/spez", "u/name/extra"} {
		if _, err := NormalizeUsername(input); err == nil {
			t.Errorf("NormalizeUsername(%q) unexpectedly succeeded", input)
		}
	}
}
