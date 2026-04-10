package main

import (
	"fmt"
	"os"

	"github.com/ipfs/kubo/repo/fsrepo/migrations"
)

// maxLegacyRepoVersion is the last repo version that requires ipfs-update.
// Kubo v0.37+ migrated to repo version 17 and embedded migration tooling,
// making ipfs-update unnecessary.
const maxLegacyRepoVersion = 16

// exitIfBuiltinUpdateAvailable checks the IPFS repo version and exits
// with guidance to use `ipfs update` when Kubo v0.37+ is detected.
func exitIfBuiltinUpdateAvailable() {
	repoVer, err := migrations.RepoVersion("")
	if err != nil {
		return
	}
	if repoVer <= maxLegacyRepoVersion {
		return
	}

	fmt.Fprintf(os.Stderr, "IPFS repo version %d (> %d) detected, indicating Kubo v0.37 or later.\n", repoVer, maxLegacyRepoVersion)
	fmt.Fprintf(os.Stderr, "ipfs-update is no longer needed. Use `ipfs update --help` instead.\n")
	os.Exit(1)
}
