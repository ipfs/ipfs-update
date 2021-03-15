package lib

import (
	"context"
	"os"

	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/whyrusleeping/stump"
)

func CheckMigration(ctx context.Context, fetcher migrations.Fetcher) error {
	stump.Log("checking if repo migration is needed...")

	oldVer, err := migrations.RepoVersion("")
	if os.IsNotExist(err) {
		stump.VLog("  - no prexisting repo to migrate")
		return nil
	}

	stump.VLog("  - old repo version is %d", oldVer)

	newVer, err := migrations.IpfsRepoVersion(ctx)
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
