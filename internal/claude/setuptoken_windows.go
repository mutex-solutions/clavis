//go:build windows

package claude

import "errors"

// ErrSetupTokenUnreadable reports that the token could not be read
// automatically. On Windows clavis never tries, so it is returned for every
// call and the caller asks for a pasted token.
var ErrSetupTokenUnreadable = errors.New("could not read the token from claude setup-token output")

// RunSetupToken is not available on Windows: driving an interactive child needs
// a ConPTY, which this build does not carry. Run `claude setup-token` yourself
// and pass the printed token with --key-stdin.
func RunSetupToken(claudePath string) (string, error) {
	return "", errors.New("automatic token capture is not supported on Windows; run `claude setup-token` and pass the token with --key-stdin")
}
