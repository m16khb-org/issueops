// Package projectdoc 패키지는 프로젝트 프로필과 프로젝트 신호 DTO를 소유한다.
// 저장소를 스캔해 얻은 언어, 도구, 명령 근거를 문서 생성 계층에 전달하는
// contract 계층 패키지다.
package projectdoc

// ProjectProfile 타입은 저장소 스캔으로 추론한 프로젝트 요약이다. 버전 관리
// 정보, 언어, 패키지 관리자, 프로젝트 유형, 프레임워크, monorepo 여부와 그 근거를
// 담는다.
type ProjectProfile struct {
	VCS             ProjectVCSProfile `json:"vcs"`
	Languages       []string          `json:"languages"`
	PackageManagers []string          `json:"package_managers,omitempty"`
	ProjectTypes    []string          `json:"project_types,omitempty"`
	Frameworks      []string          `json:"frameworks,omitempty"`
	Monorepo        bool              `json:"monorepo"`
	Evidence        []string          `json:"evidence,omitempty"`
}

// ProjectVCSProfile 타입은 프로젝트의 버전 관리 도구와 원격 호스팅 정보다.
// RemoteHost와 RemoteName은 원격이 없으면 비어 있다.
type ProjectVCSProfile struct {
	Provider   string `json:"provider"`
	Hosting    string `json:"hosting"`
	RemoteHost string `json:"remote_host,omitempty"`
	RemoteName string `json:"remote_name,omitempty"`
}
