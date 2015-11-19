package main

import (
	"bufio"
	"fmt"
	"os/exec"

	api "github.com/ipfs/go-ipfs-api"
	. "github.com/whyrusleeping/stump"
)

func GetVersions(ipfspath string) ([]string, error) {
	rc, err := Fetch(ipfspath + "/go-ipfs/versions")
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
	// try checking a locally running daemon first
	sh := api.NewShell("http://localhost:5001")
	v, _, err := sh.Version()
	if err == nil {
		return v, nil
	}

	VLog("daemon check failed: %s", err)

	_, err = exec.LookPath("ipfs")
	if err != nil {
		return "none", nil
	}

	// try running the ipfs binary in the users path
	out, err := exec.Command("ipfs", "version", "-n").CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("version check failed: %s - %s", string(out), err)
	}

	return string(out), nil
}

func GetLatestVersion(ipfspath string) (string, error) {
	vs, err := GetVersions(ipfspath)
	if err != nil {
		return "", err
	}

	return vs[len(vs)-1], nil
}
