---
name: 2026-10-19-herdr-claim
description: Caution record for a solved false case or recurring risk.
---

# Herdr 인계의 두 번째 전달 관측은 수신 세션이 claim하기 전에 기록해야 한다

- Date: 2026-10-19
- Kind: `caution`
- Source: IssueOps #570 Herdr handoff
- Summary: trace handoff-delivery의 manual 관측은 lease가 released이고 generation이 관측값과 같을 때만 받는다. 수신 세션이 인계문을 받자마자 replace·reseed·claim을 끝내면 전송 뒤 관측이 거부되고, Herdr status는 launcher runtime_id·machine_id를 내놓지 않는다.
- Context: #570 인계에서 herdr agent prompt 직후 수신 omp 세션이 수 초 안에 generation 2로 claim했다. 그 뒤 두 번째 관측은 'manual handoff delivery observation requires the exact released direct execution generation'으로 거부됐다(internal/domain/issueops/manual_handoff.go:13-16, released 상태와 generation 일치를 요구). 첫 관측도 'delivery observation launcher runtime_id is required'로 한 번 거부됐는데, herdr 0.9.5의 status 출력에는 runtime·machine·server ID가 없다.
- Resolution: 인계문 전송 전에 수신 프로세스 정보를 채울 수 없으니, 두 번째 관측은 agent prompt 영수증을 받은 즉시 기록하고 수신 process 필드는 그 시점에 관측 가능한 값만 넣는다. 거부되면 같은 입력을 재전송하지 말고 execution status의 holder session_process가 띄운 프로세스와 PID·시작 시각·실행 파일이 같은지 확인해 인계 성공을 판정한다(claim handler가 owner_claimed를 남긴다). runtime_id는 소켓을 연 herdr server 프로세스의 PID·시작 시각(lsof로 HERDR_SOCKET_PATH 보유 확인), machine_id는 IOPlatformUUID, server_id는 HERDR_SOCKET_PATH처럼 실제 관측값으로 채우고 추측값을 쓰지 않는다.
