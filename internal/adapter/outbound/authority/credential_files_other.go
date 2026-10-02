//go:build !(aix || android || darwin || dragonfly || freebsd || hurd || illumos || ios || linux || netbsd || openbsd || solaris)

package authority

import (
	"context"
	"errors"
)

var errCredentialPlatform = errors.New("authority credential files require a unix platform")

func (CredentialFiles) Write(context.Context, string, string) (string, error) {
	return "", errCredentialPlatform
}

func (CredentialFiles) Read(context.Context, string) (string, string, error) {
	return "", "", errCredentialPlatform
}
