package lib

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	api "github.com/ipfs/go-ipfs-api"
	util "github.com/ipfs/ipfs-update/util"
	stump "github.com/whyrusleeping/stump"
)

func GetVersions(ipfspath, dist string) ([]string, error) {
	rc, err := util.Fetch(ipfspath + "/" + dist + "/versions")
	if err != nil {
		return nil, err
	}
	defer rc.Close()

	var out []string
	scan := bufio.NewScanner(rc)
	for scan.Scan() {
		out = append(out, scan.Text())
	}

	return out, nil
}

func GetCurrentVersion() (string, error) {
	fix := func(s string) string {
		if !strings.HasPrefix(s, "v") {
			s = "v" + s
		}
		return s
	}

	// try checking a locally running daemon first
	apiurl, err := util.ApiEndpoint(util.IpfsDir())
	if err == nil {
		sh := api.NewShell(apiurl)
		v, _, err := sh.Version()
		if err == nil {
			return fix(v), nil
		}
	}

	stump.VLog("daemon check failed: %s", err)

	_, err = exec.LookPath("ipfs")
	if err != nil {
		return "none", nil
	}

	// try running the ipfs binary in the users path
	out, err := exec.Command("ipfs", "version", "-n").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("version check failed: %s - %s", string(out), err)
	}

	return fix(strings.Trim(string(out), " \n\t")), nil
}

func GetLatestVersion(ipfspath, dist string) (string, error) {
	vs, err := GetVersions(ipfspath, dist)
	if err != nil {
		return "", err
	}
	var latest string
	for i := len(vs) - 1; i >= 0; i-- {
		if !strings.Contains(vs[i], "-dev") {
			latest = vs[i]
			break
		}
	}
	if latest == "" {
		return "", fmt.Errorf("couldnt find a non dev version in the list")
	}
	return vs[len(vs)-1], nil
}
