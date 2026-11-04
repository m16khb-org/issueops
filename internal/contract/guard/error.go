package guard

type BlockedError struct {
	Findings []Finding
}

func (e BlockedError) Error() string {
	if len(e.Findings) == 0 {
		return "guard check blocked"
	}
	return "guard check blocked: " + e.Findings[0].Rule
}

func IsGuardBlocked(err error) bool {
	_, ok := err.(BlockedError) //nolint:errorlint // exit code 3 maps only a direct guard block; TestIsGuardBlockedMatchesDirectErrorOnly pins it
	return ok
}
