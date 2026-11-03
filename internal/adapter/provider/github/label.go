package github

import "context"

// AddLabels runs `gh issue|pr edit --add-label` (add only) and reads the
// labels back.
func (Provider) AddLabels(ctx context.Context, repo, targetURL, kind string, labels []string) ([]string, error) {
	type payload struct {
		Labels []struct {
			Name string `json:"name"`
		} `json:"labels"`
	}
	return addEdit(ctx, repo, targetURL, kind, "--add-label", "labels", labels, func(p payload) []string {
		names := make([]string, 0, len(p.Labels))
		for _, label := range p.Labels {
			names = append(names, label.Name)
		}
		return names
	})
}
