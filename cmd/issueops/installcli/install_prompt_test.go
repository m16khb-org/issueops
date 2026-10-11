package installcli

import (
	"bufio"
	"bytes"
	"errors"
	"strings"
	"testing"
	"testing/iotest"
)

func TestPromptInstallChoicesAppliesAnswersAndKeepsDefaultsOnBlank(t *testing.T) {
	cases := []struct {
		name                 string
		projectLocal, dryRun bool
		pathMode, input      string
		want                 installInteractiveChoices
		wantApplyPrompt      bool
	}{
		{"blank answers keep project and apply values and default PATH to auto", true, false, "manual", "\n\n\n",
			installInteractiveChoices{ProjectLocal: true, DryRun: false, PathMode: "auto"}, true},
		{"yes, skip, decline apply becomes dry run", false, false, "auto", "y\n3\nn\n",
			installInteractiveChoices{ProjectLocal: true, DryRun: true, PathMode: "skip"}, true},
		{"no overrides incoming project-local, manual alias, yes apply", true, false, "auto", "n\nM\nyes\n",
			installInteractiveChoices{ProjectLocal: false, DryRun: false, PathMode: "manual"}, true},
		{"already dry run never asks to apply", false, true, "auto", "on\ns\n",
			installInteractiveChoices{ProjectLocal: true, DryRun: true, PathMode: "skip"}, false},
		{"auto spelled out", false, true, "skip", "\nauto\n",
			installInteractiveChoices{ProjectLocal: false, DryRun: true, PathMode: "auto"}, false},
		{"unknown apply answer counts as no", false, false, "auto", "\n1\nmaybe\n",
			installInteractiveChoices{ProjectLocal: false, DryRun: true, PathMode: "auto"}, true},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			got, err := promptInstallChoices(tc.projectLocal, tc.dryRun, tc.pathMode, strings.NewReader(tc.input), &out)
			if err != nil {
				t.Fatal(err)
			}
			if got != tc.want {
				t.Fatalf("choices = %+v, want %+v", got, tc.want)
			}
			if has := strings.Contains(out.String(), "Apply changes now?"); has != tc.wantApplyPrompt {
				t.Fatalf("apply prompt shown = %v, want %v\n%s", has, tc.wantApplyPrompt, out.String())
			}
		})
	}
}

func TestPromptInstallChoicesRejectsBadInput(t *testing.T) {
	cases := []struct {
		name, input, wantErr string
	}{
		{"invalid path choice", "\n7\n", `invalid PATH setup choice "7"`},
		{"eof before project answer", "", "interactive input ended before Enable project-local files? [y/N]:"},
		{"eof before path answer", "y\n", "interactive input ended before Select PATH setup [1]:"},
		{"eof before apply answer", "y\n1\n", "interactive input ended before Apply changes now? [Y/n]:"},
	}
	for _, tc := range cases {
		t.Run(tc.name, func(t *testing.T) {
			var out bytes.Buffer
			_, err := promptInstallChoices(false, false, "auto", strings.NewReader(tc.input), &out)
			if err == nil || !strings.Contains(err.Error(), tc.wantErr) {
				t.Fatalf("err = %v, want %q", err, tc.wantErr)
			}
		})
	}
}

func TestPromptLineAcceptsFinalAnswerWithoutNewlineAndPropagatesReadErrors(t *testing.T) {
	var out bytes.Buffer
	got, err := promptLine(bufio.NewReader(strings.NewReader("  yes  ")), &out, "Q? ")
	if err != nil || got != "yes" || out.String() != "Q? " {
		t.Fatalf("got %q, %v, prompt %q", got, err, out.String())
	}
	if _, err := promptLine(bufio.NewReader(strings.NewReader("   ")), &out, "Q? "); err == nil {
		t.Fatal("whitespace-only final line must count as ended input")
	}
	boom := errors.New("boom")
	if _, err := promptLine(bufio.NewReader(iotest.ErrReader(boom)), &out, "Q? "); !errors.Is(err, boom) {
		t.Fatalf("read error = %v, want %v", err, boom)
	}
}

func TestYesAnswerAcceptsOnlyAffirmativeSpellings(t *testing.T) {
	for _, yes := range []string{"y", "Y", " yes ", "YES", "1", "true", "On"} {
		if !yesAnswer(yes) {
			t.Errorf("yesAnswer(%q) = false", yes)
		}
	}
	for _, no := range []string{"", "n", "no", "0", "false", "off", "yep"} {
		if yesAnswer(no) {
			t.Errorf("yesAnswer(%q) = true", no)
		}
	}
}
