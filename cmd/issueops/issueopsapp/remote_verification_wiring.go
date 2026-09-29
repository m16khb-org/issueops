package issueopsapp

import (
	"issueops/cmd/issueops/issueopscli"
	outbound "issueops/internal/adapter/outbound/remoteverification"
	app "issueops/internal/application/remoteverification"
)

func newRemoteVerifier() app.Service { return app.Service{Reader: outbound.Reader{}} }
func newRemoteVerificationHandlers() issueopscli.RemoteVerification {
	service := newRemoteVerifier()
	return issueopscli.RemoteVerification{Child: service.Child, Verify: service.Verify, VerifyContext: service.VerifyContext, Merged: service.Merged}
}
