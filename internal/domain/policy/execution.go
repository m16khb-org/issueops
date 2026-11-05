package policy

import (
	"strings"
	"time"
)

const defaultCommandTimeout = 30 * time.Second

// CommandTimeout preserves the request contract: an omitted timeout is displayed
// with the default duration, but is invalid until the inbound adapter supplies it.
func CommandTimeout(raw string) (time.Duration, bool) {
	duration, err := time.ParseDuration(raw)
	if raw == "" {
		duration = defaultCommandTimeout
	}
	return duration, err == nil
}

// CommandEnvironment filters environ (KEY=VALUE entries) down to the variables whose
// names appear in allowlist, preserving environ order. An empty allowlist yields an empty environment.
func CommandEnvironment(environ, allowlist []string) []string {
	allowed := map[string]bool{}
	for _, name := range CleanEnvAllowlist(allowlist) {
		allowed[name] = true
	}
	env := []string{}
	for _, entry := range environ {
		name, _, ok := strings.Cut(entry, "=")
		if ok && allowed[name] {
			env = append(env, entry)
		}
	}
	return env
}
