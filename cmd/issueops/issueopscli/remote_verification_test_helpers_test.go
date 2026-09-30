package issueopscli

import (
	outbound "issueops/internal/adapter/outbound/remoteverification"
	app "issueops/internal/application/remoteverification"
)

func testRemoteVerifier() app.Service { return app.Service{Reader: outbound.Reader{}} }
func testRemoteVerificationHandlers() RemoteVerification {
	service := testRemoteVerifier()
	return RemoteVerification{Child: service.Child, Verify: service.Verify, VerifyContext: service.VerifyContext, Merged: service.Merged}
}
