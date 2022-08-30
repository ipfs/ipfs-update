package config

import "encoding/json"

// CurrentAppName is the name of this application
const CurrentAppName = "ipfs-update"

// CurrentVersionNumber is the current application's version literal
var CurrentVersionNumber string

// CurrentCommit is the current git commit, if it is available.
// It might not be currently available, but it might be later if we
// add a Makefile and set it as a ldflag in the Makefile.
var CurrentCommit string

// ../version.json
type VersionFile struct {
	Version string `json:"version"`
}

func SetVersionNumber(versionFile []byte) error {
	manifest := VersionFile{}
	err := json.Unmarshal(versionFile, &manifest)
	if err == nil {
		CurrentVersionNumber = manifest.Version
	}
	return err
}
