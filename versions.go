package main

import (
	"bufio"
	"fmt"
	"os/exec"
	"strings"

	api "github.com/ipfs/go-ipfs-api"
	util "github.com/ipfs/ipfs-update/util"
	stump "github.com/whyrusleeping/stump"
)

func GetVersions(ipfspath string) ([]string, error) {
	rc, err := util.Fetch(ipfspath + "/go-ipfs/versions")
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
	sh := api.NewShell("http://localhost:5001")
	v, _, err := sh.Version()
	if err == nil {
		return fix(v), nil
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

func GetLatestVersion(ipfspath string) (string, error) {
	vs, err := GetVersions(ipfspath)
	if err != nil {
		return "", err
	}

	return vs[len(vs)-1], nil
}
