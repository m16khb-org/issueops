package issueopsintent

import "testing"

func TestMateriallyDifferentIntent(t *testing.T) {
	for _, test := range []struct {
		raw, interpreted string
		want             bool
	}{
		{"add login", "build authentication system with session management", true},
		{"add login button to the page", "implement a login button on the main page", true},
		{"add login feature for users to sign in", "add login feature for users to sign in with email", true},
		{"add", "add login feature for users", true},
		{"add login feature for users", "add", true},
		{"add login feature for all users", "add login feature for all users", false},
	} {
		if got := MateriallyDifferentIntent(test.raw, test.interpreted); got != test.want {
			t.Errorf("MateriallyDifferentIntent(%q, %q) = %v, want %v", test.raw, test.interpreted, got, test.want)
		}
	}
}

func TestIntentTokenRules(t *testing.T) {
	for _, word := range []string{"the", "a", "an", "please", "좀", "해주세요"} {
		if !intentStopWord(word) {
			t.Errorf("%q should be a stop word", word)
		}
	}
	if intentStopWord("login") {
		t.Fatal("login should be retained")
	}
	for _, word := range []string{"add", "login", "feature", "for", "users", "to", "sign", "in"} {
		if !intentTokenSet("add login feature for users to sign in")[word] {
			t.Errorf("%q missing from token set", word)
		}
	}
}

func TestCleanTextValues(t *testing.T) {
	for _, test := range []struct {
		name        string
		input, want []string
	}{
		{name: "empty"},
		{name: "single", input: []string{"hello"}, want: []string{"hello"}},
		{name: "trim and deduplicate", input: []string{"  hello  ", "hello"}, want: []string{"hello"}},
		{name: "filter empty and NUL", input: []string{"", "a\x00b", "c"}, want: []string{"c"}},
		{name: "multiple non-goals", input: []string{"no auth", "no db"}, want: []string{"no auth", "no db"}},
	} {
		t.Run(test.name, func(t *testing.T) {
			got := CleanTextValues(test.input)
			if len(got) != len(test.want) {
				t.Fatalf("clean values = %q, want %q", got, test.want)
			}
			for index := range got {
				if got[index] != test.want[index] {
					t.Fatalf("clean values = %q, want %q", got, test.want)
				}
			}
		})
	}
}
