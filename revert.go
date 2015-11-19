package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"

	. "github.com/whyrusleeping/stump"
)

func revertOldBinary(oldpath, version string) {
	stashpath := filepath.Join(ipfsDir(), "old-bin", "ipfs-"+version)
	rnerr := Move(stashpath, oldpath)
	if rnerr != nil {
		Log("error replacing binary after install fail: ", rnerr)
		Log("sorry :(")
		Log("your old ipfs binary should still be located at: ", stashpath)
	}
}

func selectRevertBin() (string, error) {
	oldbinpath := filepath.Join(ipfsDir(), "old-bin")
	_, err := os.Stat(oldbinpath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("No prior binary found at: %s", oldbinpath)
	}

	entries, err := ioutil.ReadDir(oldbinpath)
	if err != nil {
		return "", err
	}

	switch len(entries) {
	case 0:
		return "", fmt.Errorf("no prior binary found")
	case 1:
		return filepath.Join(oldbinpath, entries[0].Name()), nil
	default:
	}

	for i, e := range entries {
		if e.Name() == "path-old" {
			entries = append(entries[:i], entries[i+1:]...)
			break
		}
	}

	Log("found multiple old binaries:")
	for i, bin := range entries {
		Log("%d) %s", i+1, bin.Name())
	}

	Log("install which? ")
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		n, err := strconv.Atoi(scan.Text())
		if err != nil || n < 1 || n > len(entries) {
			Log("please enter a number in the range 1-%d", len(entries))
			continue
		}

		Log("installing %s...", entries[n-1].Name())
		return filepath.Join(oldbinpath, entries[n-1].Name()), nil
	}
	return "", fmt.Errorf("failed to select binary")
}
