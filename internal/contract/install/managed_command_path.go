package install

// ManagedCommandPathPlan 타입은 관리 대상 명령 파일 채택 계획이다. 채택을 수행하는 쪽은
// 파일시스템을 만지지만 계획을 읽고 결과로 옮기는 쪽은 그 구현을 알 필요가 없다.
// Path는 명령 경로, Target은 가리킬 대상, BackupPath는 기존 파일 백업 위치이고,
// 나머지 플래그는 채택 승인, 예정, 수행, 확정, 롤백 상태를 나타낸다.
type ManagedCommandPathPlan struct {
	Path              string
	Target            string
	BackupPath        string
	AdoptionApproved  bool
	WouldAdopt        bool
	Adopted           bool
	Committed         bool
	RolledBack        bool
	RollbackAvailable bool
	BackupRetained    bool
}
