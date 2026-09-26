package apidoc

import (
	"encoding/json"
	"os"

	app "issueops/internal/application/apidoc"
)

var (
	ErrReviewGateFailed     = app.ErrReviewGateFailed
	ErrReviewResultRequired = app.ErrReviewResultRequired
	ErrStaticGateFailed     = app.ErrStaticGateFailed
)

var ResolveTarget = func(target string) string {
	if target != "" {
		return target
	}
	cwd, err := os.Getwd()
	if err != nil {
		return "."
	}
	return cwd
}

func printJSON(value any) error {
	encoder := json.NewEncoder(os.Stdout)
	encoder.SetIndent("", "  ")
	return encoder.Encode(value)
}
