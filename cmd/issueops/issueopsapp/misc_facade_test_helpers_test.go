package issueopsapp

import (
	"errors"

	app "issueops/internal/application/apidoc"
)

func isAPIDocReviewGateError(err error) bool {
	return errors.Is(err, app.ErrReviewGateFailed)
}

func isAPIDocStaticGateError(err error) bool {
	return errors.Is(err, app.ErrStaticGateFailed)
}
