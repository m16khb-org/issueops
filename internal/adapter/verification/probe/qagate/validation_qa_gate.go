package qagate

import (
	selfverify "issueops/internal/contract/selfverify"
	"slices"

	"fmt"
	"path/filepath"
	"strings"
	"time"
)

func validateQAGateWithDeps(root string, deps docsValidationDeps) selfverify.StepResult {
	deps = deps.withDefaults()
	started := time.Now()
	errs := qaGateRequiredDocErrors(root, deps)
	skills, err := deps.listSkills(root)
	if err != nil {
		errs = append(errs, "list skills: "+err.Error())
	}
	for _, want := range []string{"atomic-commit-push", "self-augment"} {
		if !slices.Contains(skills, want) {
			errs = append(errs, "missing shared skill "+want)
		}
	}
	for _, skill := range skills {
		errs = append(errs, qaGateSkillErrors(root, skill, deps)...)
	}
	errs = append(errs, validateMermaidDocsWithDeps(root, deps)...)
	return assertionStep("QA gate", started, errs)
}

func qaGateRequiredDocErrors(root string, deps docsValidationDeps) []string {
	errs := []string{}
	requiredDocs := map[string][]string{
		filepath.Join(root, "GENIUS_THINK.md"):                                {"천재적 사고", "Mermaid"},
		filepath.Join(root, "skills", "self-augment", "SELF_AUGMENTATION.md"): {"Self-augmentation", "95"},
		filepath.Join(root, "skills", "self-verify", "SKILL.md"):              {"Self-verification", "95"},
		filepath.Join(root, ".issueops", "TESTING.md"):                        {"testing/unit-and-contract.md"},
		filepath.Join(root, ".issueops", "testing", "unit-and-contract.md"): {
			"Well-structured tests",
			"Poorly-structured tests",
		},
	}
	for path, needles := range requiredDocs {
		b, err := deps.readFile(path)
		if err != nil {
			errs = append(errs, "missing QA doc "+path)
			continue
		}
		text := string(b)
		for _, needle := range needles {
			if !strings.Contains(text, needle) {
				errs = append(errs, fmt.Sprintf("%s missing %q", path, needle))
			}
		}
	}
	return errs
}

func qaGateSkillErrors(root, skill string, deps docsValidationDeps) []string {
	skillDir := filepath.Join(root, "skills", skill)
	skillMD := filepath.Join(skillDir, "SKILL.md")
	b, err := deps.readFile(skillMD)
	if err != nil {
		return []string{"missing skill file " + skillMD}
	}
	var errs []string
	text := string(b)
	if !strings.Contains(text, "\nname:") && !strings.HasPrefix(text, "---\nname:") {
		errs = append(errs, "skill missing name frontmatter "+skill)
	}
	if !strings.Contains(text, "\ndescription:") {
		errs = append(errs, "skill missing description frontmatter "+skill)
	}
	if !deps.exists(filepath.Join(skillDir, "agents", "openai.yaml")) {
		errs = append(errs, "skill missing agents/openai.yaml "+skill)
	}
	return errs
}
