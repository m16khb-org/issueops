package projectdocs

import (
	"fmt"
	"path"
	"strings"
	"time"

	projectdocscontract "issueops/internal/contract/projectdocs"
	projectdocdomain "issueops/internal/domain/projectdoc"
)

type AppendEffects interface {
	Path(root, rel string) string
	Exists(path string) bool
	EnsureDir(path string) error
	Write(path, content string) error
	Render(kind, name, description string, request projectdocscontract.ProjectDocsAppendRequest, now time.Time) string
	Now() time.Time
}

func Append(root string, request projectdocscontract.ProjectDocsAppendRequest, effects AppendEffects) (projectdocscontract.ProjectDocsAppendResult, error) {
	plan, err := projectdocdomain.PlanAppend(projectdocdomain.AppendInput{Kind: request.Kind, Title: request.Title, Summary: request.Summary})
	if err != nil {
		return projectdocscontract.ProjectDocsAppendResult{}, err
	}
	family, ok := projectdocdomain.FamilyByModuleDir(plan.ModuleDir)
	if !ok {
		return projectdocscontract.ProjectDocsAppendResult{}, fmt.Errorf("no family owns module dir %q", plan.ModuleDir)
	}
	now := effects.Now()
	date := now.Format("2006-01-02")
	dirRel := path.Join(projectdocdomain.ProjectDocsDir, family.ModuleDir)
	if err := effects.EnsureDir(effects.Path(root, dirRel)); err != nil {
		return projectdocscontract.ProjectDocsAppendResult{}, err
	}
	base := date + "-" + plan.Slug + ".md"
	rel := path.Join(dirRel, base)
	for number := 2; effects.Exists(effects.Path(root, rel)); number++ {
		rel = path.Join(dirRel, fmt.Sprintf("%s-%s-%d.md", date, plan.Slug, number))
	}
	filePath := effects.Path(root, rel)
	description, _ := projectdocdomain.RecordMetaDescription(family.ModuleDir)
	content := effects.Render(plan.Kind, strings.TrimSuffix(path.Base(rel), ".md"), description, request, now)
	if err := effects.Write(filePath, content); err != nil {
		return projectdocscontract.ProjectDocsAppendResult{}, err
	}
	return projectdocscontract.ProjectDocsAppendResult{
		OK: true, Kind: "project_docs_append", RecordKind: plan.Kind, RepoRoot: root,
		RelPath: rel, Path: filePath, GeneratedAt: effects.Now().Format(time.RFC3339),
		BytesAppended: len([]byte(content)), SHA256: projectdocdomain.SHA256Hex(content),
	}, nil
}
