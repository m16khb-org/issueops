const harnessBin = "./bin/issueops"
const issueopsLifecycleContract = {"schema_version":1,"events":{"session_compact":{"subcommand":"post-compact","accepted_only":false},"session_start":{"subcommand":"session-start","accepted_only":false},"session_switch":{"subcommand":"session-start","accepted_only":false}},"message":{"custom_type":"issueops:project-docs","display":false,"trigger_turn":false},"warning":"issueops lifecycle hook failed","session_env":{"variable":"ISSUEOPS_OMP_SESSION_ID","agent_kind":"main","events":["session_start","session_switch"]}}

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

function exportIssueopsSessionEnv(eventName, ctx) {
  const sessionEnv = issueopsLifecycleContract.session_env
  if (!sessionEnv.events.includes(eventName)) return
  if (ctx.agent?.kind !== sessionEnv.agent_kind) return
  process.env[sessionEnv.variable] = ctx.sessionManager.getSessionId()
}

export default function agentHarness(pi) {
  for (const [eventName, rule] of Object.entries(issueopsLifecycleContract.events)) {
    pi.on(eventName, (event, ctx) => {
      if (rule.accepted_only && !event.accepted) return
      exportIssueopsSessionEnv(eventName, ctx)
      return runIssueopsLifecycle(pi, rule, ctx)
    })
  }
}
