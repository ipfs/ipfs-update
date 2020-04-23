package config

// CurrentAppName is the name of this application
const CurrentAppName = "ipfs-update"

// CurrentVersionNumber is the current application's version literal
const CurrentVersionNumber = "1.6.1-dev"

// CurrentCommit is the current git commit, if it is available.
// It might not be currently available, but it might be later if we
// add a Makefile and set it as a ldflag in the Makefile.
var CurrentCommit string

func GetUserAgent() string {
	ua := CurrentAppName + "/" + CurrentVersionNumber

	if CurrentCommit != "" {
		ua += "-" + CurrentCommit
	}

	return ua
}
