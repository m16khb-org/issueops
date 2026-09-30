package issueopslease

import "fmt"

type ResumeBeginAuthority struct {
	Pending     bool
	LeaseSame   bool
	BindingSame bool
}

func ValidateResumeBeginAuthority(authority ResumeBeginAuthority) error {
	if authority.Pending || !authority.LeaseSame || !authority.BindingSame {
		return fmt.Errorf("execution resume authority changed before intent persistence")
	}
	return nil
}
