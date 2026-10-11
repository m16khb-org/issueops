package orca

import (
	"context"
	"crypto/sha256"
	"fmt"
	"path/filepath"
	"strings"
	"time"

	deliverycontract "issueops/internal/contract/issueops"
	"issueops/internal/port"
)

type ExecutionProvisioner struct {
	client executionClient
	// terminalSettleBudget과 terminalSettleInterval이 0이면 기본 상수를 쓴다.
	// 테스트가 대기 상한을 밀리초로 줄여 실제로 기다리지 않게 하는 지점이다.
	terminalSettleBudget   time.Duration
	terminalSettleInterval time.Duration
}

func firstNonEmpty(values ...string) string {
	for _, value := range values {
		if strings.TrimSpace(value) != "" {
			return strings.TrimSpace(value)
		}
	}
	return ""
}

type executionClient interface {
	Probe(context.Context, port.OrcaProbeRequest) (port.OrcaProbeResult, error)
	ListWorktrees(context.Context, string) ([]port.OrcaWorktree, error)
	CreateWorktree(context.Context, port.OrcaCreateWorktreeRequest) (port.OrcaWorktree, error)
	CreateTerminal(context.Context, port.OrcaCreateTerminalRequest) (port.OrcaTerminal, error)
	ListRuns(context.Context) ([]port.OrcaRun, error)
	CreateRun(context.Context, port.OrcaCreateRunRequest) (port.OrcaRun, error)
	CurrentRun(context.Context) (*port.OrcaRun, error)
	UseRun(context.Context, string) (port.OrcaRun, error)
	CreateTask(context.Context, port.OrcaCreateTaskRequest) (port.OrcaTask, error)
	Dispatch(context.Context, port.OrcaDispatchRequest) (port.OrcaDispatch, error)
	SendTerminalPrompt(context.Context, string, string, string) (port.OrcaPromptReceipt, error)
}

type executionDeliveryIdentityClient interface {
	DeliveryIdentity(context.Context) (port.ExecutionOrcaDeliveryIdentity, error)
	ShowRequest(context.Context, string) (port.OrcaRequestObservation, error)
}

type executionInventoryClient interface {
	listTerminalsInventory(context.Context, string) (executionTerminalInventory, error)
	listRunTasksInventory(context.Context, string, ...string) (executionTaskInventory, error)
	showDispatchInventory(context.Context, string) (executionDispatchInventory, error)
}

type executionRunInventoryClient interface {
	listRunsInventory(context.Context) (executionRunInventory, error)
	currentRunInventory(context.Context) (executionCurrentRunInventory, error)
}

type executionTerminalDetailClient interface {
	showTerminalInventory(context.Context, string) (executionTerminalDetailInventory, error)
}

type executionTerminalInventory struct {
	RuntimeID string
	Rows      []port.OrcaTerminal
	Complete  bool
}

type executionTerminalDetailInventory struct {
	RuntimeID     string
	Terminal      port.OrcaTerminal
	PaneRuntimeID *int
}

type executionTaskInventory struct {
	RuntimeID string
	Rows      []port.OrcaTask
}

type executionRunInventory struct {
	RuntimeID string
	Rows      []port.OrcaRun
}

type executionCurrentRunInventory struct {
	RuntimeID string
	Run       *port.OrcaRun
}

type executionDispatchInventory struct {
	RuntimeID string
	Dispatch  *port.OrcaDispatch
}

func NewExecution() *ExecutionProvisioner {
	return NewExecutionClient(New())
}

func NewExecutionClient(client executionClient) *ExecutionProvisioner {
	return &ExecutionProvisioner{client: client}
}

func (p *ExecutionProvisioner) Probe(ctx context.Context, req port.ExecutionOrcaProbeRequest) (port.ExecutionOrcaProbeResult, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaProbeResult{Code: "orca_adapter_unavailable"}, nil
	}
	result, err := p.client.Probe(ctx, port.OrcaProbeRequest{Repo: req.Repo, Agent: req.Host, Provider: req.Provider})
	return port.ExecutionOrcaProbeResult{Available: result.Available, Ready: result.Ready, Code: result.Code}, err
}

func (p *ExecutionProvisioner) InspectDeliveryIdentity(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaDeliveryIdentity, error) {
	client, ok := p.client.(executionDeliveryIdentityClient)
	if !ok {
		return port.ExecutionOrcaDeliveryIdentity{}, fmt.Errorf("Orca delivery identity observation is unavailable")
	}
	identity, err := client.DeliveryIdentity(ctx)
	if err != nil {
		return port.ExecutionOrcaDeliveryIdentity{}, err
	}
	if req.Prepared == nil || strings.TrimSpace(identity.RuntimeID) != strings.TrimSpace(req.Prepared.RuntimeID) {
		return port.ExecutionOrcaDeliveryIdentity{}, fmt.Errorf("Orca delivery runtime identity changed")
	}
	terminal, err := p.resolveIntentTerminal(ctx, req)
	if err != nil {
		return port.ExecutionOrcaDeliveryIdentity{}, err
	}
	identity.TerminalPTYID = terminal.PTYID
	identity.TerminalHandle = terminal.Handle
	return identity, nil
}

func (p *ExecutionProvisioner) ObserveRequest(ctx context.Context, requestID string) (port.OrcaRequestObservation, error) {
	if err := deliverycontract.ValidateOrcaRequestID(requestID); err != nil {
		return port.OrcaRequestObservation{}, err
	}
	client, ok := p.client.(executionDeliveryIdentityClient)
	if !ok {
		return port.OrcaRequestObservation{}, fmt.Errorf("Orca request observation is unavailable")
	}
	return client.ShowRequest(ctx, requestID)
}

func (p *ExecutionProvisioner) InspectDeliveryDispatch(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentReceipt, bool, error) {
	if err := deliverycontract.ValidateOrcaRequestID(req.RetryRequestID); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, false, fmt.Errorf("Orca dispatch delivery inspection requires a durable request ID")
	}
	terminal, err := p.resolveIntentTerminal(ctx, req)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, false, err
	}
	client, err := p.intentInventoryClient()
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, false, err
	}
	inventory, err := client.showDispatchInventory(ctx, req.TaskID)
	if err != nil {
		return port.ExecutionOrcaIntentReceipt{}, false, err
	}
	if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, false, err
	}
	if inventory.Dispatch == nil {
		return port.ExecutionOrcaIntentReceipt{}, false, nil
	}
	dispatch := *inventory.Dispatch
	if err := validateExecutionObservedDispatch(dispatch, req.Prepared.RuntimeID, req.TaskID, terminal.Handle, req.RetryRequestID); err != nil {
		return port.ExecutionOrcaIntentReceipt{}, false, err
	}
	return port.ExecutionOrcaIntentReceipt{
		TaskID: dispatch.TaskID, DispatchID: dispatch.ID, RequestID: dispatch.RequestID,
		TerminalPTYID: terminal.PTYID, TerminalHandle: terminal.Handle,
	}, true, nil
}

func (p *ExecutionProvisioner) InspectIntent(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	if p == nil || p.client == nil {
		return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("Orca client is unavailable")
	}
	if err := validateExecutionIntentInspectionRequest(req); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	switch req.Stage {
	case port.ExecutionOrcaIntentWorktree:
		return p.inspectIntentWorktree(ctx, req)
	case port.ExecutionOrcaIntentTerminal:
		return p.inspectIntentTerminal(ctx, req)
	case port.ExecutionOrcaIntentRun:
		return p.inspectIntentRun(ctx, req)
	case port.ExecutionOrcaIntentRunBind:
		return p.inspectIntentRunBind(ctx, req)
	case port.ExecutionOrcaIntentTask:
		return p.inspectIntentTask(ctx, req)
	case port.ExecutionOrcaIntentDispatch:
		return p.inspectIntentDispatch(ctx, req)
	default:
		return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("unsupported Orca execution intent stage %q", req.Stage)
	}
}

func (p *ExecutionProvisioner) inspectIntentWorktree(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	rows, err := p.client.ListWorktrees(ctx, req.Workspace.SourceRoot)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
	for _, row := range rows {
		if !samePath(row.Path, req.Workspace.Root) && strings.TrimSpace(row.Comment) != req.Marker {
			continue
		}
		if err := validateExecutionWorktree(row, req.Workspace, req.Probe); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		receipt := executionWorkspaceReceipt(req.Workspace, row)
		candidates = append(candidates, port.ExecutionOrcaIntentReceipt{Workspace: &receipt})
	}
	return executionIntentInventory(candidates), nil
}

func (p *ExecutionProvisioner) inspectIntentTerminal(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	client, err := p.intentInventoryClient()
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	inventory, err := client.listTerminalsInventory(ctx, req.Prepared.WorktreeID)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
	for _, row := range inventory.Rows {
		if strings.TrimSpace(row.Title) != req.Marker && strings.TrimSpace(row.StableTabTitle) != req.Marker {
			continue
		}
		if err := validateExecutionIntentTerminal(row, *req.Prepared, req.Marker); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		candidates = append(candidates, port.ExecutionOrcaIntentReceipt{TerminalPTYID: row.PTYID})
	}
	return executionIntentInventory(candidates), nil
}

func (p *ExecutionProvisioner) inspectIntentRun(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	client, err := p.runInventoryClient()
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	inventory, err := client.listRunsInventory(ctx)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
	for _, row := range inventory.Rows {
		if strings.TrimSpace(row.Objective) != req.Marker {
			continue
		}
		if err := validateExecutionIntentRun(row, *req.Prepared, req.Marker); err != nil {
			return port.ExecutionOrcaIntentInventory{}, err
		}
		candidates = append(candidates, port.ExecutionOrcaIntentReceipt{RunID: row.ID})
	}
	return executionIntentInventory(candidates), nil
}

func (p *ExecutionProvisioner) inspectIntentRunBind(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	client, err := p.runInventoryClient()
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	inventory, err := client.currentRunInventory(ctx)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if inventory.Run == nil || inventory.Run.ID != req.RunID {
		return executionIntentInventory(nil), nil
	}
	if err := validateExecutionIntentRun(*inventory.Run, *req.Prepared, req.Marker); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	return executionIntentInventory([]port.ExecutionOrcaIntentReceipt{{RunID: inventory.Run.ID, RunBound: true}}), nil
}

func (p *ExecutionProvisioner) inspectIntentTask(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	client, err := p.intentInventoryClient()
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	inventory, err := client.listRunTasksInventory(ctx, req.RunID, "--brief")
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	title := executionTaskTitle(req.Marker, req.Launch.PromptSHA256)
	candidates := make([]port.ExecutionOrcaIntentReceipt, 0, 1)
	for _, row := range inventory.Rows {
		candidateTitle := strings.TrimSpace(row.Title)
		if candidateTitle != title {
			continue
		}
		if err := validateExecutionIntentTask(row, *req.Prepared, req.RunID, candidateTitle, req.Workspace.Branch); err != nil {
			return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("Orca owner task candidate does not match the sealed intent")
		}
		candidates = append(candidates, port.ExecutionOrcaIntentReceipt{TaskID: row.ID})
	}
	return executionIntentInventory(candidates), nil
}

func (p *ExecutionProvisioner) inspectIntentDispatch(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.ExecutionOrcaIntentInventory, error) {
	client, err := p.intentInventoryClient()
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	inventory, err := client.showDispatchInventory(ctx, req.TaskID)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if inventory.Dispatch == nil {
		return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{}, AuthoritativeZero: true}, nil
	}
	dispatch := *inventory.Dispatch
	if req.Probe.Host == "omo" {
		return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("Orca Omo prompt delivery is unproven after dispatch")
	}
	if strings.TrimSpace(req.RetryRequestID) == "" {
		return port.ExecutionOrcaIntentInventory{}, fmt.Errorf("Orca dispatch candidate requires a durable request ID")
	}
	terminal, err := p.resolveIntentTerminal(ctx, req)
	if err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	if err := validateExecutionObservedDispatch(dispatch, req.Prepared.RuntimeID, req.TaskID, terminal.Handle, req.RetryRequestID); err != nil {
		return port.ExecutionOrcaIntentInventory{}, err
	}
	return port.ExecutionOrcaIntentInventory{Candidates: []port.ExecutionOrcaIntentReceipt{{
		TaskID: dispatch.TaskID, DispatchID: dispatch.ID, RequestID: dispatch.RequestID,
		TerminalPTYID: terminal.PTYID, TerminalHandle: terminal.Handle,
	}}}, nil
}

// executionTerminalSettleBudget과 executionTerminalSettleInterval은 Orca가 만든
// 터미널의 UI 상태가 반영될 때까지 다시 읽는 상한과 간격이다.
//
// Orca는 터미널을 만든 뒤 탭 제목을 비동기로 설정한다. 마커는 그 탭 제목에 있고
// (터미널 제목은 에이전트가 자기 상태로 덮어쓴다), StableTabTitle은 visualLayouts
// 응답에서 온다. 실측에서 생성 08:32:59.706 → 검증 실패 08:33:02.607로 3초 창에
// 걸렸고, 그 창 때문에 `prepare --mode orca --confirm`이 실환경에서 한 번도
// 완주하지 못했다(이슈 #169, #70·#71이 미검증으로 남긴 경로).
//
// 상한을 실측보다 넉넉하게 잡는 것은 비대칭 때문이다: 넘으면 종전과 같은
// terminal_identity_mismatch로 떨어지므로 과하게 잡아도 손해가 없고, 부족하면
// 지금과 같다. nativeProcessProbeTimeout(execution_process.go)이 같은 성격의
// 상수 선례다.
const (
	executionTerminalSettleBudget   = 12 * time.Second
	executionTerminalSettleInterval = 400 * time.Millisecond
)

func (p *ExecutionProvisioner) reconcileCreatedTerminal(ctx context.Context, created port.OrcaTerminal, prepared port.ExecutionOrcaWorkspaceReceipt, marker string) (port.OrcaTerminal, error) {
	if err := validateExecutionIntentTerminal(created, prepared, marker); err == nil {
		return created, nil
	}
	client, ok := p.client.(executionInventoryClient)
	if !ok || (strings.TrimSpace(created.PTYID) == "" && strings.TrimSpace(created.Handle) == "") {
		return port.OrcaTerminal{}, fmt.Errorf("Orca owner terminal does not match the sealed intent")
	}
	// 반복하는 것은 조회다. CreateTerminal은 이미 한 번 실행됐고 다시 부르지
	// 않는다 — 실패한 mutation을 재시도하면 #90의 잔여물 문제를 되풀이한다.
	budget, interval := p.terminalSettleWindow()
	deadline := time.Now().Add(budget)
	attempts, lastErr := 0, error(nil)
	for {
		attempts++
		inventory, err := client.listTerminalsInventory(ctx, prepared.WorktreeID)
		if err != nil {
			return port.OrcaTerminal{}, err
		}
		candidate, err := executionSoleCreatedTerminal(inventory.Rows, created)
		if err != nil {
			// 같은 생성 identity를 가진 행이 둘 이상이면 어느 터미널을 봉인해야
			// 하는지 알 수 없다. 시간으로 해소되는 상태가 아니므로 즉시
			// fail-closed한다.
			return port.OrcaTerminal{}, err
		}
		if candidate == nil {
			// create 응답과 terminal inventory 반영 사이에는 짧은 비동기 창이
			// 있다. #190 실측에서 이 부재가 기존 12초 제목 대기를 건너뛰고
			// 약 1초 만에 실패시켰다. mutation은 반복하지 않고 같은 bounded
			// 조회 창에서 PTY 또는 handle 행이 나타나는지만 기다린다.
			lastErr = fmt.Errorf("Orca owner terminal is absent")
		} else {
			// created identity로 고른 행이므로 완화된 marker 규칙을 쓴다.
			// create 응답 자체(위 빠른 경로)에는 적용하지 않는다 — 그 응답에는
			// PTY가 아직 없을 수 있고, inventory 대기와 중복 검출이 필요하다.
			lastErr = validateExecutionResolvedTerminal(*candidate, prepared, marker)
		}
		if lastErr == nil {
			return *candidate, nil
		}
		if !time.Now().Add(interval).Before(deadline) {
			break
		}
		select {
		case <-ctx.Done():
			return port.OrcaTerminal{}, ctx.Err()
		case <-time.After(interval):
		}
	}
	// 몇 번 얼마나 기다렸는지를 남긴다. 조용한 재시도는 다음 사람이 같은 타이밍
	// 문제를 처음부터 다시 발견하게 만든다.
	return port.OrcaTerminal{}, fmt.Errorf("%w (attempt %d over %s waiting for the Orca tab title to settle)",
		lastErr, attempts, budget)
}

// terminalSettleWindow는 대기 상한과 간격을 돌려준다. 필드가 0이면 기본 상수를
// 쓴다 — 테스트가 상한을 밀리초 단위로 줄여 12초를 실제로 기다리지 않게 하려고
// 오버라이드 지점을 둔다.
func (p *ExecutionProvisioner) terminalSettleWindow() (time.Duration, time.Duration) {
	budget, interval := executionTerminalSettleBudget, executionTerminalSettleInterval
	if p.terminalSettleBudget > 0 {
		budget = p.terminalSettleBudget
	}
	if p.terminalSettleInterval > 0 {
		interval = p.terminalSettleInterval
	}
	return budget, interval
}

// executionSoleCreatedTerminal은 생성 응답의 PTY가 있으면 PTY를, 아직 없으면
// handle을 써서 최대 한 행을 고른다. handle은 inventory에서 PTY를 회수하기
// 위한 일시적 selector이며 durable receipt에는 남기지 않는다. 부재는 생성 직후
// inventory가 따라오는 동안 생길 수 있으므로 nil,nil이고, 중복은 identity
// ambiguity라 즉시 오류다.
func executionSoleCreatedTerminal(rows []port.OrcaTerminal, created port.OrcaTerminal) (*port.OrcaTerminal, error) {
	ptyID := strings.TrimSpace(created.PTYID)
	handle := strings.TrimSpace(created.Handle)
	var candidate *port.OrcaTerminal
	for index := range rows {
		row := rows[index]
		matches := ptyID != "" && strings.TrimSpace(row.PTYID) == ptyID
		if ptyID == "" {
			matches = handle != "" && strings.TrimSpace(row.Handle) == handle
		}
		if !matches {
			continue
		}
		if candidate != nil {
			return nil, fmt.Errorf("Orca owner terminal inventory is ambiguous")
		}
		candidate = &row
	}
	return candidate, nil
}

func (p *ExecutionProvisioner) intentInventoryClient() (executionInventoryClient, error) {
	client, ok := p.client.(executionInventoryClient)
	if !ok {
		return nil, fmt.Errorf("Orca execution intent inventory is unavailable")
	}
	return client, nil
}

func (p *ExecutionProvisioner) runInventoryClient() (executionRunInventoryClient, error) {
	client, ok := p.client.(executionRunInventoryClient)
	if !ok {
		return nil, fmt.Errorf("Orca Run inventory is unavailable")
	}
	return client, nil
}

func (p *ExecutionProvisioner) resolveIntentTerminal(ctx context.Context, req port.ExecutionOrcaIntentRequest) (port.OrcaTerminal, error) {
	client, err := p.intentInventoryClient()
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	inventory, err := client.listTerminalsInventory(ctx, req.Prepared.WorktreeID)
	if err != nil {
		return port.OrcaTerminal{}, err
	}
	if err := validateExecutionInventoryRuntime(inventory.RuntimeID, req.Prepared.RuntimeID); err != nil {
		return port.OrcaTerminal{}, err
	}
	var candidate *port.OrcaTerminal
	for index := range inventory.Rows {
		row := inventory.Rows[index]
		if row.PTYID != req.TerminalPTYID {
			continue
		}
		if candidate != nil {
			return port.OrcaTerminal{}, fmt.Errorf("Orca owner terminal inventory is ambiguous")
		}
		candidate = &row
	}
	if candidate == nil {
		return port.OrcaTerminal{}, fmt.Errorf("Orca owner terminal is absent")
	}
	if err := validateExecutionTerminalReceipt(*candidate, *req.Prepared); err != nil {
		return port.OrcaTerminal{}, err
	}
	return *candidate, nil
}

func executionWorkspaceReceipt(workspace port.ExecutionWorkspaceRequest, worktree port.OrcaWorktree) port.ExecutionOrcaWorkspaceReceipt {
	return port.ExecutionOrcaWorkspaceReceipt{
		Workspace: port.ExecutionWorkspaceReceipt{
			SourceRoot: workspace.SourceRoot, Root: filepath.Clean(worktree.Path), Branch: workspace.Branch,
			BaseHead: workspace.BaseHead, ParentWorktree: workspace.ParentWorktree,
			Driver: "orca", Exists: true,
		},
		RuntimeID: worktree.RuntimeID, RepoID: worktree.RepoID, WorktreeID: worktree.ID, WorktreeInstanceID: worktree.InstanceID,
	}
}

func executionIntentInventory(candidates []port.ExecutionOrcaIntentReceipt) port.ExecutionOrcaIntentInventory {
	if candidates == nil {
		candidates = []port.ExecutionOrcaIntentReceipt{}
	}
	return port.ExecutionOrcaIntentInventory{Candidates: candidates, AuthoritativeZero: len(candidates) == 0}
}

func executionTaskTitle(marker, promptSHA256 string) string {
	marker = strings.TrimSpace(marker)
	lifecycleID := ""
	for field := range strings.FieldsSeq(marker) {
		if value, ok := strings.CutPrefix(field, "lifecycle="); ok {
			lifecycleID = value
			break
		}
	}
	intentDigest := sha256.Sum256([]byte(marker + "\n" + strings.ToLower(strings.TrimSpace(promptSHA256))))
	return fmt.Sprintf("issueops-v1 lifecycle=%s intent=%x", lifecycleID, intentDigest[:8])
}
