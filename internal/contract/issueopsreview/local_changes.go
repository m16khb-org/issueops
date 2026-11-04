package issueopsreview

// LocalChangeObservation binds readiness and review classification to the same verified snapshot.
type LocalChangeObservation struct {
	Paths       []string
	Fingerprint string
	Verified    bool
}

// ChangeBaseCandidate 타입은 로컬 변경의 diff 기준으로 시도해 볼 ref 하나다.
// LiteralObject는 ref가 브랜치 이름이 아니라 전체 git 객체 ID이므로 옵션으로
// 해석되지 않게 --end-of-options 뒤에서 검증해야 함을 뜻한다.
type ChangeBaseCandidate struct {
	Ref           string
	LiteralObject bool
}

// UpstreamFetch 타입은 root 저장소의 upstream fetch 시도 결과다. Failed이면
// Stderr에 git 오류 출력이 담긴다.
type UpstreamFetch struct {
	Root   string
	Failed bool
	Stderr string
}
