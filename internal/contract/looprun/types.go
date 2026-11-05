// Package looprun 패키지는 looprun capability의 DTO를 소유한다.
//
// 값을 만들어내는 쪽은 I/O를 하지만, 결과를 읽고 전달하는 쪽은 그 구현을
// 알 필요가 없다.
package looprun

// LoopRun 타입은 검증을 통과할 때까지 반복하는 loop 계약이다. VerifyArgv는 매 시도의
// 검증 명령이고, 시도가 MaxAttempts를 넘으면 소진된다.
type LoopRun struct {
	OK            bool          `json:"ok"`
	SchemaVersion int           `json:"schema_version"`
	ID            string        `json:"id"`
	Repo          string        `json:"repo"`
	Name          string        `json:"name"`
	Goal          string        `json:"goal"`
	VerifyArgv    []string      `json:"verify_argv,omitempty"`
	MaxAttempts   int           `json:"max_attempts"`
	Status        string        `json:"status"`
	Attempts      []LoopAttempt `json:"attempts,omitempty"`
	StopReason    string        `json:"stop_reason,omitempty"`
	CreatedAt     string        `json:"created_at"`
	UpdatedAt     string        `json:"updated_at"`
}

// LoopAttempt 타입은 loop의 시도 하나다. Seq는 1부터 매기는 순번이고 Verdict와
// Evidence가 그 시도의 판정과 근거다.
type LoopAttempt struct {
	Seq      int      `json:"seq"`
	Verdict  string   `json:"verdict"`
	Evidence []string `json:"evidence"`
	At       string   `json:"at"`
}

// StartLoopRequest 타입은 loop 시작(또는 재개) 입력이다. Repo 안에서 Name으로
// loop를 식별한다.
type StartLoopRequest struct {
	Repo        string
	Name        string
	Goal        string
	VerifyArgv  []string
	MaxAttempts int
}

// RecordAttemptRequest 타입은 활성 loop에 시도 하나를 추가하는 입력이다.
type RecordAttemptRequest struct {
	Verdict  string
	Evidence []string
}

// StatusResult 타입은 loop 상태 조회 결과로, loop와 시도 횟수, 마지막 판정을 담는다.
type StatusResult struct {
	OK           bool    `json:"ok"`
	Loop         LoopRun `json:"loop"`
	AttemptCount int     `json:"attempt_count"`
	LastVerdict  string  `json:"last_verdict,omitempty"`
}

// RepoGateSummary 타입은 저장소 안의 활성 loop와 소진된 loop 개수를 센다.
type RepoGateSummary struct {
	Active    int
	Exhausted int
}
