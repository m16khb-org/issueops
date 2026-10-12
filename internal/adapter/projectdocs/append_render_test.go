package projectdocs

import (
	"testing"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
)

var appendRenderNow = time.Date(2026, time.March, 4, 23, 59, 0, 0, time.UTC)

func TestAppendFilesRenderCautionRecordWithEveryField(t *testing.T) {
	got := AppendFiles{}.Render("caution", "n", "d", projectdocscontract.ProjectDocsAppendRequest{
		Title:        "  Title  ",
		Source:       " src ",
		Summary:      " sum ",
		Context:      " ctx ",
		Resolution:   " fix ",
		Decision:     "ignored decision",
		Consequences: "ignored consequences",
		Evidence:     []string{" e1 ", "  ", "e2"},
		Alternatives: []string{" a1 ", "", "a2"},
	}, appendRenderNow)
	want := "---\nname: n\ndescription: d\n---\n\n" +
		"# Title\n\n" +
		"- Date: 2026-03-04\n" +
		"- Kind: `caution`\n" +
		"- Source: src\n" +
		"- Summary: sum\n" +
		"- Context: ctx\n" +
		"- Resolution: fix\n" +
		"- Evidence:\n  - e1\n  - e2\n" +
		"- Alternatives / rejected options:\n  - a1\n  - a2\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestAppendFilesRenderDecisionRecordWithEveryField(t *testing.T) {
	got := AppendFiles{}.Render("decision", "n", "d", projectdocscontract.ProjectDocsAppendRequest{
		Title:        "T",
		Source:       "s",
		Summary:      "sum",
		Context:      "ctx",
		Resolution:   "ignored resolution",
		Decision:     " pick ",
		Consequences: " cost ",
	}, appendRenderNow)
	want := "---\nname: n\ndescription: d\n---\n\n" +
		"# T\n\n" +
		"- Date: 2026-03-04\n" +
		"- Kind: `decision`\n" +
		"- Source: s\n" +
		"- Summary: sum\n" +
		"- Context: ctx\n" +
		"- Decision: pick\n" +
		"- Consequences: cost\n"
	if got != want {
		t.Fatalf("got:\n%q\nwant:\n%q", got, want)
	}
}

func TestAppendFilesRenderOmitsBlankOptionalFields(t *testing.T) {
	blank := projectdocscontract.ProjectDocsAppendRequest{
		Title:        "T",
		Source:       "  ",
		Summary:      "sum",
		Context:      "\t",
		Resolution:   " ",
		Decision:     " ",
		Consequences: " ",
		Evidence:     []string{" "},
		Alternatives: []string{""},
	}
	// All-blank list entries must not leave dangling "Evidence:" / "Alternatives" headers in the record.
	for kind, want := range map[string]string{
		"caution":  "# T\n\n- Date: 2026-03-04\n- Kind: `caution`\n- Summary: sum\n",
		"decision": "# T\n\n- Date: 2026-03-04\n- Kind: `decision`\n- Summary: sum\n",
	} {
		got := AppendFiles{}.Render(kind, "n", "d", blank, appendRenderNow)
		if body := got[len("---\nname: n\ndescription: d\n---\n\n"):]; body != want {
			t.Fatalf("%s: body %q, want %q", kind, body, want)
		}
	}

	none := projectdocscontract.ProjectDocsAppendRequest{Title: "T", Summary: "sum"}
	got := AppendFiles{}.Render("decision", "n", "d", none, appendRenderNow)
	if want := "---\nname: n\ndescription: d\n---\n\n# T\n\n- Date: 2026-03-04\n- Kind: `decision`\n- Summary: sum\n"; got != want {
		t.Fatalf("got %q, want %q", got, want)
	}
}
