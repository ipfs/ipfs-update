package main

import (
	"archive/zip"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	stump "github.com/whyrusleeping/stump"
)

func InstallVersion(root, v string, nocheck bool) error {
	currentVersion, err := GetCurrentVersion()
	if err != nil {
		return err
	}

	if currentVersion == "none" {
		stump.VLog("no pre-existing ipfs installation found")
	}

	stump.Log("installing ipfs version %s", v)
	tmpd, err := ioutil.TempDir("", "ipfs-update")
	if err != nil {
		return err
	}

	binpath := filepath.Join(tmpd, "ipfs-new")

	stump.Log("fetching %s binary...", v)
	err = GetBinaryForVersion(root, v, binpath)
	if err != nil {
		return err
	}

	if !nocheck {
		stump.Log("binary downloaded, verifying...")
		err = TestBinary(binpath, v)
		if err != nil {
			return err
		}
	} else {
		stump.Log("skipping tests since '--no-check' was passed")
	}

	var installPath string
	if currentVersion != "none" {
		stump.Log("stashing old binary")
		oldpath, err := StashOldBinary(currentVersion, false)
		if err != nil {
			return err
		}
		installPath = oldpath
	} else {
		// need to select installation location
		ipath, err := SelectGoodInstallLoc()
		if err != nil {
			return err
		}

		installPath = ipath
	}

	stump.Log("installing new binary to %s", installPath)
	err = InstallBinaryTo(binpath, installPath)
	if err != nil {
		// in case of error here, replace old binary
		stump.Error("Install failed: ", err)
		if currentVersion != "none" {
			revertOldBinary(installPath, currentVersion)
		}
		return err
	}

	if beforeVersion("v0.3.10", v) {
		stump.VLog("  - ipfs pre v0.3.10 does not support checking of repo version through the tool")
		stump.VLog("  - if a migration is needed, you will be prompted when starting ipfs")
	} else {
		err := CheckMigration()
		if err != nil {
			stump.Error("Migration Failed: ", err)
			revertOldBinary(installPath, currentVersion)
			return err
		}
	}

	return nil
}

func InstallBinaryTo(nbin, nloc string) error {
	err := CopyTo(nbin, nloc)
	if err != nil {
		return fmt.Errorf("error moving new binary into place: %s", err)
	}

	err = os.Chmod(nloc, 0755)
	if err != nil {
		return fmt.Errorf("error setting permissions on new binary: %s", err)
	}

	return nil
}

// StashOldBinary moves the existing ipfs binary to a backup directory
// and returns the path to the original location of the old binary
func StashOldBinary(tag string, keep bool) (string, error) {
	loc, err := exec.LookPath("ipfs")
	if err != nil {
		return "", fmt.Errorf("could not find old binary: %s", err)
	}

	ipfsdir := ipfsDir()

	olddir := filepath.Join(ipfsdir, "old-bin")
	npath := filepath.Join(olddir, "ipfs-"+tag)
	pathpath := filepath.Join(olddir, "path-old")

	err = os.MkdirAll(olddir, 0700)
	if err != nil {
		return "", fmt.Errorf("could not create dir to backup old binary: %s", err)
	}

	// write the old path of the binary to the backup dir
	err = ioutil.WriteFile(pathpath, []byte(loc), 0644)
	if err != nil {
		return "", fmt.Errorf("couldnt stash path: ", err)
	}

	f := Move
	if keep {
		f = CopyTo
	}

	stump.VLog("  - moving %s to %s", loc, npath)
	err = f(loc, npath)
	if err != nil {
		return "", fmt.Errorf("could not move old binary: %s", err)
	}

	return loc, nil
}

func GetBinaryForVersion(root, vers, target string) error {
	dir, err := ioutil.TempDir("", "ipfs-update")
	if err != nil {
		return err
	}

	stump.VLog("  - using GOOS=%s and GOARCH=%s", runtime.GOOS, runtime.GOARCH)
	finame := fmt.Sprintf("go-ipfs_%s_%s-%s.zip", vers, runtime.GOOS, runtime.GOARCH)

	ipfspath := fmt.Sprintf("%s/go-ipfs/%s/%s", root, vers, finame)

	data, err := Fetch(ipfspath)
	if err != nil {
		return err
	}

	zippath := filepath.Join(dir, finame)
	fi, err := os.Create(zippath)
	if err != nil {
		return err
	}

	stump.VLog("  - writing to", zippath)
	_, err = io.Copy(fi, data)
	if err != nil {
		return err
	}
	fi.Close()

	zipr, err := zip.OpenReader(zippath)
	if err != nil {
		return fmt.Errorf("error opening zipreader: %s", err)
	}

	defer zipr.Close()

	var bin io.ReadCloser
	for _, fis := range zipr.File {
		if fis.Name == "ipfs/ipfs" {
			rc, err := fis.Open()
			if err != nil {
				return fmt.Errorf("error extracting binary from archive: %s", err)
			}

			bin = rc
		}
	}

	if bin == nil {
		return fmt.Errorf("no ipfs binary found in downloaded archive")
	}

	stump.VLog("  - extracting binary to tempdir: ", target)
	binfi, err := os.Create(target)
	if err != nil {
		return fmt.Errorf("error opening tmp bin path '%s': %s", target, err)
	}
	defer binfi.Close()

	_, err = io.Copy(binfi, bin)
	if err != nil {
		return err
	}

	return nil
}

var errNoGoodInstall = fmt.Errorf("could not find good install location")

func SelectGoodInstallLoc() (string, error) {
	// gopath setup? first choice
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		return filepath.Join(gopath, "bin"), nil
	}

	common := []string{"/usr/local/bin"}
	for _, dir := range common {
		if canWrite(dir) && isInPath(dir) {
			return dir, nil
		}
	}

	// hrm, none of those worked. lets check home.
	home := os.Getenv("HOME")
	if home == "" {
		return "", errNoGoodInstall
	}

	homebin := filepath.Join(home, "bin")
	if canWrite(homebin) {
		return homebin, nil
	}
	return "", errNoGoodInstall
}

func isInPath(dir string) bool {
	return strings.Contains(os.Getenv("PATH"), dir)
}

func canWrite(dir string) bool {
	fi, err := ioutil.TempFile(dir, ".ipfs-update-test")
	if err != nil {
		return false
	}

	_, err = fi.Write([]byte("test"))
	if err != nil {
		return false
	}

	_ = os.Remove(fi.Name())
	return true
}
