// Package docs는 문서 색인 결과의 DTO를 소유한다.
//
// application이 색인을 구성하고 adapter가 파일을 읽는다. 결과를 전달하고
// 직렬화하는 쪽은 그 구현을 알 필요가 없다.
package docs

type DocsIndexResult struct {
	OK           bool           `json:"ok"`
	Version      string         `json:"version"`
	IssueOpsRoot string         `json:"issueops_root"`
	Docs         []DocIndexInfo `json:"docs"`
	GeneratedAt  string         `json:"generated_at"`
}

type DocIndexInfo struct {
	RelPath  string   `json:"rel_path"`
	Path     string   `json:"path"`
	Title    string   `json:"title"`
	Headings []string `json:"headings"`
	Bytes    int64    `json:"bytes"`
}
