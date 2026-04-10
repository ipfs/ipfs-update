package main

import (
	"fmt"
	"os"

	"github.com/ipfs/kubo/repo/fsrepo/migrations"
)

// delegateMinRepoVersion is the repo version above which ipfs-update
// refuses to run and points users to the built-in `ipfs update` command.
const delegateMinRepoVersion = 16

// exitIfBuiltinUpdateAvailable checks the IPFS repo version and exits
// with guidance to use `ipfs update` when the repo version indicates
// a Kubo release that ships the built-in update command.
func exitIfBuiltinUpdateAvailable() {
	repoVer, err := migrations.RepoVersion("")
	if err != nil {
		return
	}
	if repoVer <= delegateMinRepoVersion {
		return
	}

	fmt.Fprintf(os.Stderr, "IPFS repo version %d detected (> %d).\n", repoVer, delegateMinRepoVersion)
	fmt.Fprintf(os.Stderr, "Your Kubo version has a built-in update command.\n")
	fmt.Fprintf(os.Stderr, "Run `ipfs update --help` instead of ipfs-update.\n")
	os.Exit(1)
}
