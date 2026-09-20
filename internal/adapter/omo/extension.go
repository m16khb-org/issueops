package omo

import (
	"encoding/json"
	"fmt"
)

type lifecycleContract struct {
	SchemaVersion int                      `json:"schema_version"`
	Events        map[string]lifecycleRule `json:"events"`
	Message       lifecycleMessage         `json:"message"`
	Warning       string                   `json:"warning"`
}

type lifecycleRule struct {
	Subcommand   string `json:"subcommand"`
	AcceptedOnly bool   `json:"accepted_only"`
}

type lifecycleMessage struct {
	CustomType  string `json:"custom_type"`
	Display     bool   `json:"display"`
	TriggerTurn bool   `json:"trigger_turn"`
}

func omoLifecycleExtension(binPath string) string {
	encodedBin, _ := json.Marshal(binPath)
	encodedContract, _ := json.Marshal(lifecycleContract{
		SchemaVersion: 1,
		Events: map[string]lifecycleRule{
			"session_start":   {Subcommand: "session-start"},
			"session_compact": {Subcommand: "post-compact", AcceptedOnly: true},
		},
		Message: lifecycleMessage{
			CustomType:  "issueops:project-docs",
			Display:     false,
			TriggerTurn: false,
		},
		Warning: "issueops lifecycle hook failed",
	})
	return fmt.Sprintf(`const harnessBin = %s
const issueopsLifecycleContract = %s

async function runIssueopsLifecycle(pi, rule, ctx) {
  try {
    const result = await pi.exec(
      harnessBin,
      ["hook", rule.subcommand, "--repo", ctx.cwd, "--json"],
      { cwd: ctx.cwd },
    )
    if (result.code !== 0) {
      ctx.ui.notify(issueopsLifecycleContract.warning, "warning")
      return
    }
    const payload = JSON.parse(result.stdout)
    if (!payload.should_inject || !payload.compact) return
    pi.sendMessage(
      {
        customType: issueopsLifecycleContract.message.custom_type,
        content: payload.compact,
        display: issueopsLifecycleContract.message.display,
      },
      { triggerTurn: issueopsLifecycleContract.message.trigger_turn },
    )
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    ctx.ui.notify(issueopsLifecycleContract.warning + ": " + detail, "warning")
  }
}

export default function agentHarness(pi) {
  for (const [eventName, rule] of Object.entries(issueopsLifecycleContract.events)) {
    pi.on(eventName, (event, ctx) => {
      if (rule.accepted_only && !event.accepted) return
      return runIssueopsLifecycle(pi, rule, ctx)
    })
  }
}
`, encodedBin, encodedContract)
}

func LifecycleExtension(binPath string) string {
	return omoLifecycleExtension(binPath)
}
