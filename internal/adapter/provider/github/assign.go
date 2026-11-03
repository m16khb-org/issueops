package github

import "context"

// AddAssignees runs `gh issue|pr edit --add-assignee` (add only) and reads
// the assignees back.
func (Provider) AddAssignees(ctx context.Context, repo, targetURL, kind string, users []string) ([]string, error) {
	type payload struct {
		Assignees []struct {
			Login string `json:"login"`
		} `json:"assignees"`
	}
	return addEdit(ctx, repo, targetURL, kind, "--add-assignee", "assignees", users, func(p payload) []string {
		logins := make([]string, 0, len(p.Assignees))
		for _, assignee := range p.Assignees {
			logins = append(logins, assignee.Login)
		}
		return logins
	})
}
