package issueops

import (
	"reflect"
	"testing"

	issueopscontract "issueops/internal/contract/issueops"
)

func TestEvaluateHandoffMaterialScenarios(t *testing.T) {
	base := func() issueopscontract.IssueOpsHandoffSnapshot {
		return issueopscontract.IssueOpsHandoffSnapshot{
			SchemaVersion: issueopscontract.IssueOpsHandoffSchemaVersion,
			Phase:         issueopscontract.IssueOpsHandoffPhaseReleaseReadiness,
			Sealed: issueopscontract.IssueOpsHandoffSealed{
				BaseHead:       "base",
				FullHead:       "head",
				PlanDigest:     "plan",
				MaterialDigest: "material",
				Material: issueopscontract.IssueOpsHandoffMaterial{
					Purpose:           "finish task 10",
					NonGoals:          []string{"new process manager"},
					ApprovedEndpoint:  "draft PR",
					SourceRoot:        "/repo/source",
					CanonicalWorktree: "/repo/worktree",
					PlanPath:          ".issueops/plans/review-verification-efficiency.md",
					DiffDigest:        "diff",
					LifecycleState:    "plan.handoff",
					ResumeCommands:    []string{"issueops execution status --id io-1 --json"},
					ReadOnlyCommands:  []string{"git status --short"},
					Verification: []issueopscontract.IssueOpsHandoffVerification{
						{ID: "tests", Input: "head", Command: "go test ./...", Timestamp: "2026-09-20T00:00:00Z", Environment: "local", ResultLocation: "reports/tests.txt"},
					},
				},
				RequiredEvidence: []issueopscontract.IssueOpsHandoffEvidence{
					{ID: "tests", Digest: "tests-digest", Head: "head", PlanDigest: "plan", MaterialDigest: "material", InputRevision: "head"},
					{ID: "investigation", Digest: "investigation-digest", Head: "head", PlanDigest: "plan", MaterialDigest: "material", InputRevision: "plan"},
				},
			},
			Current: issueopscontract.IssueOpsHandoffCurrent{
				FullHead:       "head",
				PlanDigest:     "plan",
				MaterialDigest: "material",
				Evidence: []issueopscontract.IssueOpsHandoffEvidence{
					{ID: "tests", Digest: "tests-digest", Head: "head", PlanDigest: "plan", MaterialDigest: "material", InputRevision: "head"},
					{ID: "investigation", Digest: "investigation-digest", Head: "head", PlanDigest: "plan", MaterialDigest: "material", InputRevision: "plan"},
				},
				SharedStateRechecked: true,
			},
			Sender:        issueopscontract.IssueOpsHandoffSession{Host: "codex", SessionID: "sender", ProcessReceipt: "pid-1"},
			UserDirective: issueopscontract.IssueOpsHandoffUserDirective{MaterialVersion: 3, LatestVersion: 3, CurrentInstruction: "finish the approved task 10 scope"},
			Tasks: []issueopscontract.IssueOpsHandoffTask{
				reader("reader-a"),
				reader("reader-b"),
			},
		}
	}

	tests := []struct {
		name string
		edit func(*issueopscontract.IssueOpsHandoffSnapshot)
		want issueopscontract.IssueOpsHandoffDecision
	}{
		{
			name: "receiver absent before release allows release readiness",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {},
			want: releaseDecision(true, []string{"investigation", "tests"}, nil, nil, nil),
		},
		{
			name: "precreated receiver rejects release readiness",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Receiver = issueopscontract.IssueOpsHandoffSession{Host: "codex", SessionID: "receiver", ProcessReceipt: "pid-2"}
			},
			want: releaseDecision(false, nil, nil, nil, []string{"receiver_present_before_release"}),
		},
		{
			name: "reader2 writer1 blocks release until writer descendants terminate and shared state is rechecked",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Current.SharedStateRechecked = false
				s.Tasks = append(s.Tasks, issueopscontract.IssueOpsHandoffTask{
					ID: "writer", Classification: issueopscontract.IssueOpsHandoffTaskWriter, Owner: "agent", ExecutionHandle: "job-1",
					InputRevision: "rev-1", WriteScope: "worktree", ResultLocation: "artifact/writer.txt",
					Live: false, DescendantsLive: true,
				})
			},
			want: releaseDecision(false, []string{"investigation", "tests"}, []string{"shared_state_recheck", "task:writer"}, nil, []string{"pending_writer:writer", "live_descendant:writer", "shared_state_not_rechecked"}),
		},
		{
			name: "writer drained with shared state rechecked allows release and reuses matching evidence",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Tasks = append(s.Tasks, issueopscontract.IssueOpsHandoffTask{
					ID: "writer", Classification: issueopscontract.IssueOpsHandoffTaskWriter, Owner: "agent", ExecutionHandle: "job-1",
					InputRevision: "rev-1", WriteScope: "worktree", ResultLocation: "artifact/writer.txt",
					Live: false, Terminated: true,
				})
			},
			want: releaseDecision(true, []string{"investigation", "tests"}, nil, nil, nil),
		},
		{
			name: "reader only still requires final shared state recheck",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Current.SharedStateRechecked = false
			},
			want: releaseDecision(false, []string{"investigation", "tests"}, []string{"shared_state_recheck"}, nil, []string{"shared_state_not_rechecked"}),
		},
		{
			name: "zero task snapshot still requires final shared state recheck",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Tasks = nil
				s.Current.SharedStateRechecked = false
			},
			want: releaseDecision(false, []string{"investigation", "tests"}, []string{"shared_state_recheck"}, nil, []string{"shared_state_not_rechecked"}),
		},
		{
			name: "delayed generator is writer and blocks release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Tasks = append(s.Tasks, issueopscontract.IssueOpsHandoffTask{
					ID: "generator", Kind: issueopscontract.IssueOpsHandoffTaskKindGenerator, Classification: issueopscontract.IssueOpsHandoffTaskWriter,
					Owner: "tool", ExecutionHandle: "gen-1", InputRevision: "rev-1", WriteScope: "generated fixture",
					ResultLocation: "generated/out.go", Live: true,
				})
			},
			want: releaseDecision(false, []string{"investigation", "tests"}, []string{"task:generator"}, nil, []string{"pending_writer:generator"}),
		},
		{
			name: "cancellation resistant child blocks release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Tasks = append(s.Tasks, issueopscontract.IssueOpsHandoffTask{
					ID: "child", Classification: issueopscontract.IssueOpsHandoffTaskWriter, Owner: "child-agent", ExecutionHandle: "child-1",
					InputRevision: "rev-1", WriteScope: "child worktree", ResultLocation: "child/report.md",
					Live: true, DescendantsLive: true, CancellationRequested: true,
				})
			},
			want: releaseDecision(false, []string{"investigation", "tests"}, []string{"task:child"}, nil, []string{"pending_writer:child", "live_descendant:child", "cancellation_not_observed:child"}),
		},
		{
			name: "late callback result is quarantined after release boundary",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				receivePhase(s)
				s.LateResults = []issueopscontract.IssueOpsHandoffLateResult{
					{ID: "callback-1", SourceSessionID: "sender", InputRevision: "rev-old", ResultLocation: "mailbox/result.json", ArrivedAfterRelease: true, AttemptsSourceChange: true},
				}
			},
			want: receiveDecision(true, []string{"investigation", "tests"}, nil, []string{"callback-1"}, nil),
		},
		{
			name: "equal version user cancel stops dispatch",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.UserDirective = issueopscontract.IssueOpsHandoffUserDirective{MaterialVersion: 3, LatestVersion: 3, CurrentInstruction: "stop", Cancelled: true}
			},
			want: releaseDecision(false, nil, nil, nil, []string{"user_cancelled"}),
		},
		{
			name: "newer user scope change stops dispatch",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.UserDirective = issueopscontract.IssueOpsHandoffUserDirective{MaterialVersion: 3, LatestVersion: 4, CurrentInstruction: "reduce the scope", ScopeChanged: true}
			},
			want: releaseDecision(false, nil, nil, nil, []string{"user_scope_changed_after_material"}),
		},
		{
			name: "missing current user instruction rejects release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.UserDirective.CurrentInstruction = ""
			},
			want: releaseDecision(false, nil, nil, nil, []string{"missing_current_user_instruction"}),
		},
		{
			name: "changed head selectively rechecks head bound evidence",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Current.FullHead = "new-head"
			},
			want: releaseDecision(false, nil, []string{"evidence:investigation", "evidence:tests", "head"}, nil, []string{"head_changed"}),
		},
		{
			name: "changed plan and material reject stale material",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Current.PlanDigest = "new-plan"
				s.Current.MaterialDigest = "new-material"
			},
			want: releaseDecision(false, nil, []string{"evidence:investigation", "evidence:tests", "material", "plan"}, nil, []string{"material_changed", "plan_changed"}),
		},
		{
			name: "missing evidence selectively rechecks only missing evidence",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Current.Evidence = []issueopscontract.IssueOpsHandoffEvidence{{ID: "tests", Digest: "tests-digest", Head: "head", PlanDigest: "plan", MaterialDigest: "material", InputRevision: "head"}}
			},
			want: releaseDecision(false, []string{"tests"}, []string{"evidence:investigation"}, nil, []string{"missing_evidence:investigation"}),
		},
		{
			name: "sealed evidence binding must match sealed material before reuse",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Sealed.RequiredEvidence[0].Head = ""
				s.Sealed.RequiredEvidence[1].PlanDigest = "other-plan"
			},
			want: releaseDecision(false, nil, nil, nil, []string{"missing_required_evidence_head:tests", "stale_required_evidence_plan:investigation"}),
		},
		{
			name: "duplicate and empty evidence ids reject release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Sealed.RequiredEvidence = append(s.Sealed.RequiredEvidence, issueopscontract.IssueOpsHandoffEvidence{ID: "tests", Digest: "again", Head: "head", PlanDigest: "plan", MaterialDigest: "material", InputRevision: "head"})
				s.Current.Evidence = append(s.Current.Evidence, issueopscontract.IssueOpsHandoffEvidence{ID: "", Digest: "empty"})
			},
			want: releaseDecision(false, nil, nil, nil, []string{"duplicate_required_evidence:tests", "missing_current_evidence_id"}),
		},
		{
			name: "missing structured material rejects release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Sealed.Material.Purpose = ""
				s.Sealed.Material.ResumeCommands = nil
			},
			want: releaseDecision(false, nil, nil, nil, []string{"missing_material_purpose", "missing_resume_commands"}),
		},
		{
			name: "empty structured list entries reject release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Sealed.Material.NonGoals = []string{""}
				s.Sealed.Material.ReadOnlyCommands = []string{""}
			},
			want: releaseDecision(false, nil, nil, nil, []string{"missing_material_non_goal", "missing_read_only_command"}),
		},
		{
			name: "missing verification metadata rejects release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Sealed.Material.Verification[0].Command = ""
				s.Sealed.Material.Verification = append(s.Sealed.Material.Verification, issueopscontract.IssueOpsHandoffVerification{
					ID: "tests", Input: "head", Command: "go vet ./...", Timestamp: "2026-09-20T00:01:00Z", Environment: "local", ResultLocation: "reports/vet.txt",
				})
			},
			want: releaseDecision(false, nil, nil, nil, []string{"missing_verification_command:tests", "duplicate_verification:tests"}),
		},
		{
			name: "missing task owner handle revision and result rejects release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Tasks = append(s.Tasks, issueopscontract.IssueOpsHandoffTask{
					ID: "writer", Classification: issueopscontract.IssueOpsHandoffTaskWriter, WriteScope: "worktree", Terminated: true,
				})
			},
			want: releaseDecision(false, nil, nil, nil, []string{"missing_task_owner:writer", "missing_task_execution_handle:writer", "missing_task_input_revision:writer", "missing_task_result_location:writer"}),
		},
		{
			name: "blank and duplicate task ids reject release",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				s.Tasks = append(s.Tasks,
					issueopscontract.IssueOpsHandoffTask{ID: "", Classification: issueopscontract.IssueOpsHandoffTaskReader, Owner: "agent", ExecutionHandle: "empty", InputRevision: "rev", ResultLocation: "artifact/empty.txt", Terminated: true},
					issueopscontract.IssueOpsHandoffTask{ID: "reader-a", Classification: issueopscontract.IssueOpsHandoffTaskReader, Owner: "agent", ExecutionHandle: "dup", InputRevision: "rev", ResultLocation: "artifact/dup.txt", Terminated: true},
				)
			},
			want: releaseDecision(false, nil, nil, nil, []string{"missing_task_id", "duplicate_task:reader-a"}),
		},
		{
			name: "blank duplicate and missing late result provenance reject receive",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				receivePhase(s)
				s.LateResults = []issueopscontract.IssueOpsHandoffLateResult{
					{ID: "", SourceSessionID: "sender", InputRevision: "rev", ResultLocation: "mailbox/empty.json", ArrivedAfterRelease: true},
					{ID: "late", ArrivedAfterRelease: true},
					{ID: "late", SourceSessionID: "sender", InputRevision: "rev", ResultLocation: "mailbox/late2.json", ArrivedAfterRelease: true},
				}
			},
			want: receiveDecision(false, nil, nil, nil, []string{"missing_late_result_id", "missing_late_result_source_session:late", "missing_late_result_input_revision:late", "missing_late_result_result_location:late", "duplicate_late_result:late"}),
		},
		{
			name: "cross host same session id requires new identity",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				receivePhase(s)
				s.Receiver = issueopscontract.IssueOpsHandoffSession{Host: "claude", SessionID: "sender", ProcessReceipt: "pid-3"}
			},
			want: receiveDecision(false, nil, nil, nil, []string{"cross_host_session_id_not_portable"}),
		},
		{
			name: "direct without orca packet allowed after material and status validation",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				receivePhase(s)
				s.Execution = issueopscontract.IssueOpsHandoffExecution{Mode: issueopscontract.ExecutionModeDirect, StatusValidated: true, OrcaPacketPresent: false}
			},
			want: receiveDecision(true, []string{"investigation", "tests"}, nil, nil, nil),
		},
		{
			name: "orca recovery keeps packet requirement",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				receivePhase(s)
				s.Execution = issueopscontract.IssueOpsHandoffExecution{Mode: issueopscontract.ExecutionModeOrca, StatusValidated: true, OrcaPacketPresent: false, OrcaRecoveryAction: "resume"}
			},
			want: receiveDecision(false, nil, nil, nil, []string{"orca_packet_required"}),
		},
		{
			name: "receive before release rejects receive readiness",
			edit: func(s *issueopscontract.IssueOpsHandoffSnapshot) {
				receivePhase(s)
				s.Current.ReleaseCompleted = false
			},
			want: receiveDecision(false, nil, nil, nil, []string{"release_not_recorded"}),
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			snapshot := base()
			tt.edit(&snapshot)
			got := EvaluateHandoff(snapshot)
			if got.AllowRelease && got.AllowReceive {
				t.Fatalf("EvaluateHandoff() set both phase outputs: %#v", got)
			}
			if !reflect.DeepEqual(got, tt.want) {
				t.Fatalf("EvaluateHandoff() mismatch\n got: %#v\nwant: %#v", got, tt.want)
			}
		})
	}
}

func receivePhase(snapshot *issueopscontract.IssueOpsHandoffSnapshot) {
	snapshot.Phase = issueopscontract.IssueOpsHandoffPhaseReceiveReadiness
	snapshot.Current.ReleaseCompleted = true
	snapshot.Receiver = issueopscontract.IssueOpsHandoffSession{Host: "codex", SessionID: "receiver", ProcessReceipt: "pid-2"}
	snapshot.Execution = issueopscontract.IssueOpsHandoffExecution{Mode: issueopscontract.ExecutionModeDirect, StatusValidated: true}
}

func reader(id string) issueopscontract.IssueOpsHandoffTask {
	return issueopscontract.IssueOpsHandoffTask{
		ID: id, Classification: issueopscontract.IssueOpsHandoffTaskReader, Owner: "agent", ExecutionHandle: id + "-handle",
		InputRevision: "rev-1", ResultLocation: "artifact/" + id + ".txt", Terminated: true,
	}
}

func releaseDecision(allow bool, reuse, recheck, quarantine, reject []string) issueopscontract.IssueOpsHandoffDecision {
	return issueopscontract.IssueOpsHandoffDecision{
		Phase:            issueopscontract.IssueOpsHandoffPhaseReleaseReadiness,
		AllowRelease:     allow,
		ReusableEvidence: reuse,
		SelectiveRecheck: recheck,
		Quarantine:       quarantine,
		RejectReasons:    reject,
	}
}

func receiveDecision(allow bool, reuse, recheck, quarantine, reject []string) issueopscontract.IssueOpsHandoffDecision {
	return issueopscontract.IssueOpsHandoffDecision{
		Phase:            issueopscontract.IssueOpsHandoffPhaseReceiveReadiness,
		AllowReceive:     allow,
		ReusableEvidence: reuse,
		SelectiveRecheck: recheck,
		Quarantine:       quarantine,
		RejectReasons:    reject,
	}
}
