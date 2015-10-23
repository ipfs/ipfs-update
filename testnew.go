package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
)

func runCmd(p, bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = []string{"IPFS_PATH=" + p}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(out))
	}

	return string(out), nil
}

func TestBinary(bin, version string) error {
	staging := path.Join(ipfsDir(), "update-staging")
	err := os.MkdirAll(staging, 0755)
	if err != nil {
		return fmt.Errorf("error creating test staging directory: %s", err)
	}

	tdir, err := ioutil.TempDir(staging, "test")
	if err != nil {
		return err
	}

	err = os.MkdirAll(tdir, 0755)
	if err != nil {
		return fmt.Errorf("error creating test staging directory: %s", err)
	}

	defer func(dir string) {
		// defer cleanup, bound param to avoid mistakes
		err = os.RemoveAll(dir)
		if err != nil {
			fmt.Println("error cleaning up staging directory: ", err)
		}
	}(tdir)

	tbin := path.Join(tdir, "ipfs")
	_, err = runCmd(tdir, bin, "init")
	if err != nil {
		return err
	}

	rversion, err := runCmd(tdir, tbin, "version")
	if err != nil {
		return err
	}

	fmt.Printf("'%s'\n", version)
	if rversion != "ipfs version "+version {
		return fmt.Errorf("version didnt match")
	}

	return nil
}
