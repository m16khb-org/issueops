package gitlab

import (
	"context"
	"fmt"
	remote "issueops/internal/domain/issueopsremote"
	"issueops/internal/port"
	"strings"
)

type gitlabRecoveryItem struct {
	ID           string `json:"id"`
	IID          string `json:"iid"`
	URL          string `json:"webUrl"`
	Title        string `json:"title"`
	Body         string `json:"description"`
	WorkItemType struct {
		Name string `json:"name"`
	} `json:"workItemType"`
	Widgets []struct {
		Type   string `json:"type"`
		Parent *struct {
			ID  string `json:"id"`
			URL string `json:"webUrl"`
		} `json:"parent"`
		HasParent bool                `json:"hasParent"`
		Labels    gitlabLabelNodes    `json:"labels"`
		Assignees gitlabAssigneeNodes `json:"assignees"`
	} `json:"widgets"`
}
type gitlabRecoveryItems struct {
	Data struct {
		Project struct {
			WorkItems struct {
				Nodes    []gitlabRecoveryItem `json:"nodes"`
				PageInfo struct {
					HasNextPage bool `json:"hasNextPage"`
				} `json:"pageInfo"`
			} `json:"workItems"`
		} `json:"project"`
	} `json:"data"`
}

const gitlabRecoveryFields = `id iid webUrl title description workItemType { name } widgets { type ... on WorkItemWidgetHierarchy { hasParent parent { id webUrl } } ... on WorkItemWidgetLabels { labels { nodes { title } } } ... on WorkItemWidgetAssignees { assignees { nodes { username } } } }`
const gitlabChildLookupQuery = `query childRecovery($projectPath: ID!, $childIid: String!) { project(fullPath: $projectPath) { workItems(iid: $childIid, first: 2) { nodes { ` + gitlabRecoveryFields + ` } pageInfo { hasNextPage } } } }`
const gitlabChildSearchQuery = `query childRecoverySearch($projectPath: ID!, $marker: String!) { project(fullPath: $projectPath) { workItems(search: $marker, in: [DESCRIPTION], first: 100) { nodes { ` + gitlabRecoveryFields + ` } pageInfo { hasNextPage } } } }`

func (p Provider) FindChildCreateCandidates(ctx context.Context, req port.IssueProviderFindIssueCreateCandidatesRequest) (port.IssueProviderFindIssueCreateCandidatesResult, error) {
	var result port.IssueProviderFindIssueCreateCandidatesResult
	parts := strings.SplitN(req.ProjectAuthority, "/", 2)
	if len(parts) != 2 || req.Marker == "" {
		return result, fmt.Errorf("invalid child recovery search")
	}
	response, err := runGlabGraphQLContext[gitlabRecoveryItems](ctx, req.Repo, parts[0], gitlabChildSearchQuery, map[string]string{"projectPath": parts[1], "marker": req.Marker})
	if err != nil {
		return result, err
	}
	rows := response.Data.Project.WorkItems
	result.Truncated = rows.PageInfo.HasNextPage || len(rows.Nodes) >= 100
	for _, item := range rows.Nodes {
		if strings.Contains(item.Body, req.Marker) {
			result.Candidates = append(result.Candidates, port.IssueProviderIssueCreateCandidate{URL: item.URL, Title: item.Title, Body: item.Body})
		}
	}
	return result, nil
}

func readGitlabRecoveryItem(ctx context.Context, repo, parentURL, childURL string) (gitlabRecoveryItem, string, string, error) {
	var zero gitlabRecoveryItem
	if err := remote.ValidateChildMatchesParent(parentURL, childURL); err != nil {
		return zero, "", "", err
	}
	host, project, _, err := parseGitLabIssueURL(parentURL)
	if err != nil {
		return zero, "", "", err
	}
	_, _, iid, err := parseGitLabWorkItemURL(childURL)
	if err != nil {
		return zero, "", "", err
	}
	response, err := runGlabGraphQLContext[gitlabRecoveryItems](ctx, repo, host, gitlabChildLookupQuery, map[string]string{"projectPath": project, "childIid": iid})
	if err != nil {
		return zero, "", "", err
	}
	rows := response.Data.Project.WorkItems
	if rows.PageInfo.HasNextPage || len(rows.Nodes) != 1 || rows.Nodes[0].IID != iid || rows.Nodes[0].URL != childURL || rows.Nodes[0].ID == "" {
		return zero, "", "", fmt.Errorf("child lookup identity mismatch")
	}
	return rows.Nodes[0], host, project, nil
}

func (p Provider) ReadChild(ctx context.Context, repo, parentURL, childURL string) (port.ChildSnapshot, error) {
	var result port.ChildSnapshot
	item, _, _, err := readGitlabRecoveryItem(ctx, repo, parentURL, childURL)
	if err != nil {
		return result, err
	}
	result = port.ChildSnapshot{URL: item.URL, Title: item.Title, Body: item.Body, TypeVerified: strings.EqualFold(item.WorkItemType.Name, "Task")}
	hierarchyFound := false
	for _, widget := range item.Widgets {
		if widget.Type == "HIERARCHY" {
			hierarchyFound = true
			if widget.HasParent && (widget.Parent == nil || !sameGitLabIssueSnapshotIdentity(widget.Parent.URL, parentURL)) {
				return result, fmt.Errorf("child has a different or unobservable parent")
			}
			if widget.Parent != nil {
				if !sameGitLabIssueSnapshotIdentity(widget.Parent.URL, parentURL) {
					return result, fmt.Errorf("child has a different parent")
				}
				result.HierarchyVerified = true
			}
		}
		for _, label := range widget.Labels.Nodes {
			result.Labels = append(result.Labels, label.Title)
		}
		for _, assignee := range widget.Assignees.Nodes {
			result.Assignees = append(result.Assignees, assignee.Username)
		}
	}
	if !hierarchyFound {
		return result, fmt.Errorf("child hierarchy widget missing")
	}
	return result, nil
}
func (p Provider) AttachChild(ctx context.Context, repo, parentURL, childURL string) error {
	snapshot, err := p.ReadChild(ctx, repo, parentURL, childURL)
	if err != nil {
		return err
	}
	if snapshot.HierarchyVerified {
		return nil
	}
	item, host, project, err := readGitlabRecoveryItem(ctx, repo, parentURL, childURL)
	if err != nil {
		return err
	}
	_, _, parentIID, err := parseGitLabIssueURL(parentURL)
	if err != nil {
		return err
	}
	parent, err := runGlabGraphQLContext[gitlabParentIssueResponse](ctx, repo, host, gitlabParentIssueQuery, map[string]string{"projectPath": project, "parentIid": parentIID})
	if err != nil {
		return err
	}
	parentID := parent.Data.Project.Issue.ID
	if parentID == "" {
		return fmt.Errorf("parent lookup identity missing")
	}
	query := `mutation childRecoveryAttach($parentId: WorkItemID!, $childId: WorkItemID!) { workItemHierarchyAddChildrenItems(input: { id: $parentId, childrenIds: [$childId] }) { errors } }`
	response, err := runGlabGraphQLContext[gitlabHierarchyAddResponse](ctx, repo, host, query, map[string]string{"parentId": parentID, "childId": item.ID})
	if err != nil {
		return err
	}
	if len(response.Data.WorkItemHierarchyAddChildrenItems.Errors) > 0 {
		return fmt.Errorf("child attach failed: %s", strings.Join(response.Data.WorkItemHierarchyAddChildrenItems.Errors, ", "))
	}
	return nil
}
