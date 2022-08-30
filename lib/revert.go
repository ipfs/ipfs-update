package lib

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"text/tabwriter"
	"time"

	"github.com/ipfs/ipfs-update/util"
	"github.com/ipfs/kubo/repo/fsrepo/migrations"
	"github.com/whyrusleeping/stump"
)

func revertOldBinary(oldpath, version string) {
	ipfsDir, err := migrations.CheckIpfsDir("")
	if err != nil {
		stump.Log("Error reverting")
		stump.Log("failed to replace binary after install fail")
		stump.Log("could not find location of old binary:", err)
		return
	}

	stashpath := filepath.Join(ipfsDir, "old-bin", "ipfs-"+version)
	err = util.Move(stashpath, oldpath)
	if err != nil {
		stump.Log("Error reverting")
		stump.Log("failed to replace binary after install fail:", err)
		stump.Log("sorry :(")
		stump.Log("your old ipfs binary should still be located at:", stashpath)
		stump.Log("try: `mv %q %q`", stashpath, oldpath)
	}
}

func SelectRevertBin() (string, error) {
	ipfsDir, err := migrations.CheckIpfsDir("")
	if err != nil {
		return "", err
	}
	oldbinpath := filepath.Join(ipfsDir, "old-bin")
	_, err = os.Stat(oldbinpath)
	if os.IsNotExist(err) {
		return "", fmt.Errorf("no prior binary found at: %s", oldbinpath)
	}

	entries, err := ioutil.ReadDir(oldbinpath)
	if err != nil {
		return "", err
	}

	for i, e := range entries {
		if e.Name() == "path-old" {
			entries = append(entries[:i], entries[i+1:]...)
			break
		}
	}

	switch len(entries) {
	case 0:
		return "", fmt.Errorf("no prior binary found")
	case 1:
		return filepath.Join(oldbinpath, entries[0].Name()), nil
	default:
	}

	stump.Log("found multiple old binaries:")
	tw := tabwriter.NewWriter(stump.LogOut, 6, 4, 4, ' ', 0)
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
			stump.Log("please enter a number in the range 1-%d (0 to exit)", len(entries))
			continue
		}

		stump.Log("installing %s...", entries[n-1].Name())
		return filepath.Join(oldbinpath, entries[n-1].Name()), nil
	}
	return "", fmt.Errorf("failed to select binary")
}
