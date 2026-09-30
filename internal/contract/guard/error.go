package guard

type GuardBlockedError struct {
	Findings []GuardFinding
}

func (e GuardBlockedError) Error() string {
	if len(e.Findings) == 0 {
		return "guard check blocked"
	}
	return "guard check blocked: " + e.Findings[0].Rule
}

func IsGuardBlocked(err error) bool {
	_, ok := err.(GuardBlockedError)
	return ok
}
