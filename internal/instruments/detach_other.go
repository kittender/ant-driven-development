//go:build !unix

package instruments

import "os/exec"

func detach(*exec.Cmd) {}
