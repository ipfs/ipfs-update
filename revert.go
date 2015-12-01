package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"
	"time"

	util "github.com/ipfs/ipfs-update/util"
	stump "github.com/whyrusleeping/stump"
)

func revertOldBinary(oldpath, version string) {
	stashpath := filepath.Join(util.IpfsDir(), "old-bin", "ipfs-"+version)
	rnerr := util.Move(stashpath, oldpath)
	if rnerr != nil {
		stump.Log("error replacing binary after install fail: ", rnerr)
		stump.Log("sorry :(")
		stump.Log("your old ipfs binary should still be located at: ", stashpath)
	}
}

func selectRevertBin() (string, error) {
	oldbinpath := filepath.Join(util.IpfsDir(), "old-bin")
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

	stump.Log("found multiple old binaries:")
	tw := tabwriter.NewWriter(os.Stdout, 6, 4, 4, ' ', 0)
	for i, bin := range entries {
		fmt.Fprintf(tw, "%d)\t%s\t%s\n", i+1, bin.Name(), bin.ModTime().Format(time.ANSIC))
	}
	tw.Flush()

	stump.Log("install which? (0 to exit)")
	scan := bufio.NewScanner(os.Stdin)
	for scan.Scan() {
		n, err := strconv.Atoi(scan.Text())
		if n == 0 {
			return "", fmt.Errorf("exiting at user request")
		}
		if err != nil || n < 1 || n > len(entries) {
			stump.Log("please enter a number in the range 1-%d", len(entries))
			continue
		}

		stump.Log("installing %s...", entries[n-1].Name())
		return filepath.Join(oldbinpath, entries[n-1].Name()), nil
	}
	return "", fmt.Errorf("failed to select binary")
}
