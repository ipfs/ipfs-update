package lib

import (
	"fmt"
	"os/exec"
	"strings"
)

// CurrentIpfsVersion returns the version of the currently running or installed
// ipfs executable.
func CurrentIpfsVersion() (string, error) {
	// try checking a locally running daemon first
	_, ver, err := ApiShell("")
	if err != nil {
		_, err = exec.LookPath("ipfs")
		if err != nil {
			return "none", nil
		}

		// try running the ipfs binary in the users path
		out, err := exec.Command("ipfs", "version", "-n").CombinedOutput()
		if err != nil {
			return "", fmt.Errorf("version check failed: %s - %s", string(out), err)
		}

		ver = strings.Trim(string(out), " \n\t")
	}

	if !strings.HasPrefix(ver, "v") {
		ver = "v" + ver
	}

	return ver, nil
}
