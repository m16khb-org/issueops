const harnessBin = "./bin/issueops"
const issueopsLifecycleContract = {"schema_version":1,"hook":{"root":"session-start","child":"subagent-start"},"cache_limit":512,"timeout_ms":10000,"warning":"issueops lifecycle hook failed"}

async function runIssueopsHook(subcommand, repo, input) {
  const proc = Bun.spawn([harnessBin, "hook", subcommand, "--repo", repo, "--json"], {
    cwd: repo,
    stdin: input === undefined ? "ignore" : "pipe",
    stdout: "pipe",
    stderr: "ignore",
  })
  if (input !== undefined) {
    proc.stdin.write(input)
    proc.stdin.end()
  }
  let timedOut = false
  const timer = setTimeout(() => {
    timedOut = true
    proc.kill()
  }, issueopsLifecycleContract.timeout_ms)
  try {
    const [stdout, code] = await Promise.all([Bun.readableStreamToText(proc.stdout), proc.exited])
    if (timedOut) throw new Error("timed out")
    if (code !== 0) throw new Error("exit " + code)
    const payload = JSON.parse(stdout)
    return payload.should_inject && payload.compact ? payload.compact : ""
  } finally {
    clearTimeout(timer)
  }
}

async function loadIssueopsCatalog(ctx, event) {
  try {
    let info = {}
    try {
      info = (await ctx.session.get({ sessionID: event.sessionID })) ?? {}
    } catch {}
    const repo = info.location?.directory || ctx.location.directory
    if (info.parentID) {
      return await runIssueopsHook(issueopsLifecycleContract.hook.child, repo, JSON.stringify({ agent_type: event.agent }))
    }
    return await runIssueopsHook(issueopsLifecycleContract.hook.root, repo)
  } catch (error) {
    const detail = error instanceof Error ? error.message : String(error)
    console.error(issueopsLifecycleContract.warning + ": " + detail)
    return ""
  }
}

export default {
  id: "issueops",
  setup(ctx) {
    const catalogs = new Map()
    ctx.session.hook("context", async (event) => {
      let catalog = catalogs.get(event.sessionID)
      if (catalog === undefined) {
        catalog = loadIssueopsCatalog(ctx, event)
        catalogs.set(event.sessionID, catalog)
        if (catalogs.size > issueopsLifecycleContract.cache_limit) {
          catalogs.delete(catalogs.keys().next().value)
        }
      }
      const text = await catalog
      if (text) event.system.push({ type: "text", text })
    })
  },
}
