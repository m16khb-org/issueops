package apidoc

import (
	"issueops/cmd/issueops/apidoc/reviewprompt"
	"issueops/internal/adapter/outbound/apidoc/reviewfiles"
	"issueops/internal/adapter/preflight"
	app "issueops/internal/application/apidoc"
)

func testAPIDocService() app.Service {
	files := reviewfiles.Files{GitCmd: preflight.GitCmd}
	return app.Service{
		Static: app.StaticService{Effects: app.StaticEffects{NormalizeFiles: reviewfiles.Normalize, TrackedFiles: files.Tracked, StagedFiles: files.Staged, Mode: reviewfiles.Mode, ReadFile: reviewfiles.ReadFile}},
		Reviewer: app.ReviewService{Effects: app.ReviewEffects{
			NormalizeFiles: reviewfiles.Normalize, TrackedFiles: files.Tracked, StagedFiles: files.Staged, Input: files.Input,
			ExtraPrompt: func(o app.ReviewOptions) (string, error) { return reviewfiles.ExtraPrompt(o.Repo, o.PromptFile) },
			Evidence:    reviewfiles.Evidence, BuildPrompt: reviewprompt.Build, Schema: reviewprompt.Schema, ReadResult: reviewfiles.ReadResult,
		}},
	}
}
