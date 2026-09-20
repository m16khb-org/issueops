const harnessBin = "./bin/issueops"
const issueopsLifecycleContract = {"schema_version":1,"events":{"session_compact":{"subcommand":"post-compact","accepted_only":true},"session_start":{"subcommand":"session-start","accepted_only":false}},"message":{"custom_type":"issueops:project-docs","display":false,"trigger_turn":false},"warning":"issueops lifecycle hook failed"}

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
