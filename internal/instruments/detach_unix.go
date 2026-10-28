//go:build unix

package instruments

import (
	"os/exec"
	"syscall"
)

// detach starts the command in a new session, without a controlling
// terminal, so it cannot write to the console the pilot reports to.
func detach(cmd *exec.Cmd) {
	cmd.SysProcAttr = &syscall.SysProcAttr{Setsid: true}
}
