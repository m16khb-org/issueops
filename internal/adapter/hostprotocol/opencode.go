package hostprotocol

import (
	"encoding/json"
	"fmt"
)

type opencodeContract struct {
	SchemaVersion int               `json:"schema_version"`
	Hook          opencodeHookNames `json:"hook"`
	CacheLimit    int               `json:"cache_limit"`
	TimeoutMS     int               `json:"timeout_ms"`
	Warning       string            `json:"warning"`
}

type opencodeHookNames struct {
	Root  string `json:"root"`
	Child string `json:"child"`
}

// OpencodeLifecyclePlugin returns the one canonical opencode v2 plugin module
// for a harness binary.
//
// opencode v2 has no session-start event, and its `context` session hook runs
// before every agent-loop model call, including the first one after a
// compaction. The plugin therefore reads the catalog once per session, caches
// it, and appends it to the system prompt of every call; no post-compact hook
// is needed. The event has no parent id, so one `ctx.session.get` per session
// tells a subagent apart. The plugin never writes process.env: one opencode
// server hosts many sessions, and opencode itself exports
// OPENCODE_SESSION_ID to each session's shell tools.
func OpencodeLifecyclePlugin(binPath string) string {
	encodedBin, _ := json.Marshal(binPath)
	encodedContract, _ := json.Marshal(opencodeContract{
		SchemaVersion: 1,
		Hook:          opencodeHookNames{Root: "session-start", Child: "subagent-start"},
		CacheLimit:    512,
		TimeoutMS:     10000,
		Warning:       "issueops lifecycle hook failed",
	})
	return fmt.Sprintf(`const harnessBin = %s
const issueopsLifecycleContract = %s

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
`, encodedBin, encodedContract)
}
