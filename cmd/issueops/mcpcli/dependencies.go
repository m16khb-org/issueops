package mcpcli

import (
	"errors"
	app "issueops/internal/application/selfverify"
)

func isSelfVerificationGateError(err error) bool {
	return errors.Is(err, app.ErrSelfVerificationGateFailed)
}
