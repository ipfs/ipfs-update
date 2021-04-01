package lib

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/whyrusleeping/stump"
)

func checkMigration(ctx context.Context, fetcher migrations.Fetcher, binPath string) error {
	stump.Log("checking if repo migration is needed...")

	oldVer, err := migrations.RepoVersion("")
	if os.IsNotExist(err) {
		stump.VLog("  - no prexisting repo to migrate")
		return nil
	}

	stump.VLog("  - old repo version is %d", oldVer)

	newVer, err := ipfsRepoVersion(ctx, binPath)
	if err != nil {
		stump.Log("Failed to check new binary repo version.")
		stump.VLog("Reason: ", err)
		stump.Log("This is not an error.")
		stump.Log("This just means that you may have to manually run the migration")
		stump.Log("You will be prompted to do so upon starting the ipfs daemon if necessary")
		return nil
	}

	stump.VLog("  - repo version of new binary is %d", newVer)

	if oldVer != newVer {
		stump.Log("  check complete, migration required.")
		return migrations.RunMigration(ctx, fetcher, newVer, "", true)
	}

	stump.VLog("  check complete, no migration required.")
	return nil
}

// ipfsRepoVersion returns the repo version required by the ipfs daemon
func ipfsRepoVersion(ctx context.Context, binPath string) (int, error) {
	out, err := exec.CommandContext(ctx, binPath, "version", "--repo").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("%s: %s", err, string(out))
	}

	verStr := strings.TrimSpace(string(out))
	ver, err := strconv.Atoi(verStr)
	if err != nil {
		return 0, fmt.Errorf("repo version is not an integer: %s", verStr)
	}

	return ver, nil
}
