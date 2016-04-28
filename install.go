package main

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	test "github.com/ipfs/ipfs-update/test-dist"
	util "github.com/ipfs/ipfs-update/util"
	stump "github.com/whyrusleeping/stump"
)

func (i *Install) getTmpPath() (string, error) {
	tmpd, err := ioutil.TempDir("", "ipfs-update")
	if err != nil {
		return "", err
	}

	err = os.MkdirAll(tmpd, 0777)
	if err != nil {
		return "", err
	}

	return filepath.Join(tmpd, "ipfs-new"), nil
}

func NewInstall(root, target string, nocheck bool) (*Install, error) {
	return &Install{
		TargetVers: target,
		UrlRoot:    root,
		NoCheck:    nocheck,
		BinaryName: "ipfs",
	}, nil
}

type Install struct {
	// name of binary to be installed
	BinaryName string

	TargetVers  string
	CurrentVers string

	TmpBinPath string

	StashedFromPath string

	InstallPath string

	NoCheck bool

	UrlRoot string

	// whether or not the install has succeeded
	Succeeded bool
}

func (i *Install) Run() error {
	defer i.RevertOnFailure()

	var err error
	i.CurrentVers, err = GetCurrentVersion()
	if err != nil {
		return err
	}

	if i.CurrentVers == "none" {
		stump.VLog("no pre-existing ipfs installation found")
	}

	err = i.DownloadNewBinary()
	if err != nil {
		return err
	}

	if !i.NoCheck {
		stump.Log("binary downloaded, verifying...")
		err = test.TestBinary(i.TmpBinPath, i.TargetVers)
		if err != nil {
			return err
		}
	} else {
		stump.Log("skipping tests since '--no-check' was passed")
	}

	err = i.MaybeStash()
	if err != nil {
		return err
	}

	err = i.SelectGoodInstallLoc()
	if err != nil {
		return err
	}

	stump.Log("installing new binary to %s", i.InstallPath)
	err = InstallBinaryTo(i.TmpBinPath, i.InstallPath)
	if err != nil {
		// in case of error here, replace old binary
		stump.Error("Install failed: ", err)

		return err
	}

	err = i.postInstallMigrationCheck()
	if err != nil {
		stump.Error("Migration Failed: ", err)
		return err
	}

	i.Succeeded = true
	return nil
}

func (i *Install) RevertOnFailure() {
	if i.Succeeded {
		return
	}

	stump.Log("install failed, reverting changes...")

	if i.CurrentVers != "none" && i.InstallPath != "" {
		revertOldBinary(i.InstallPath, i.CurrentVers)
	}
}

func (i *Install) MaybeStash() error {
	if i.CurrentVers != "none" {
		stump.Log("stashing old binary")
		oldpath, err := StashOldBinary(i.CurrentVers, false)
		if err != nil {
			if strings.Contains(err.Error(), "could not find old") {
				stump.Log("stash failed, no binary found.")
				stump.Log("this could be because you have a daemon running, but no ipfs binary in your path.")
				stump.Log("continuing anyways, but skipping stash")
				return nil
			}
			return err
		}
		i.StashedFromPath = filepath.Dir(oldpath)
	} else {
		stump.VLog("skipping stash, no previous install")
	}

	return nil
}

func (i *Install) postInstallMigrationCheck() error {
	if util.BeforeVersion("v0.3.10", i.TargetVers) {
		stump.VLog("  - ipfs pre v0.3.10 does not support checking of repo version through the tool")
		stump.VLog("  - if a migration is needed, you will be prompted when starting ipfs")
		return nil
	}

	return CheckMigration()
}

func InstallBinaryTo(nbin, nloc string) error {
	err := util.CopyTo(nbin, nloc)
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

	ipfsdir := util.IpfsDir()

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

	f := util.Move
	if keep {
		f = util.CopyTo
	}

	stump.VLog("  - moving %s to %s", loc, npath)
	err = f(loc, npath)
	if err != nil {
		return "", fmt.Errorf("could not move old binary: %s", err)
	}

	return loc, nil
}

func (i *Install) DownloadNewBinary() error {
	out, err := i.getTmpPath()
	if err != nil {
		return err
	}

	err = GetBinaryForVersion("go-ipfs", "ipfs", i.UrlRoot, i.TargetVers, out)
	if err != nil {
		return err
	}

	i.TmpBinPath = out
	return nil
}

func GetBinaryForVersion(distname, binnom, root, vers, out string) error {
	stump.Log("fetching %s version %s", distname, vers)
	dir, err := ioutil.TempDir("", "ipfs-update")
	if err != nil {
		return err
	}

	stump.VLog("  - using GOOS=%s and GOARCH=%s", runtime.GOOS, runtime.GOARCH)
	var archive string
	switch runtime.GOOS {
	case "windows":
		archive = "zip"
	default:
		archive = "tar.gz"
	}
	finame := fmt.Sprintf("%s_%s_%s-%s.%s", distname, vers, runtime.GOOS, runtime.GOARCH, archive)

	distpath := fmt.Sprintf("%s/%s/%s/%s", root, distname, vers, finame)

	data, err := util.Fetch(distpath)
	if err != nil {
		return err
	}

	arcpath := filepath.Join(dir, finame)
	fi, err := os.Create(arcpath)
	if err != nil {
		return err
	}

	stump.VLog("  - writing to", arcpath)
	_, err = io.Copy(fi, data)
	if err != nil {
		return err
	}
	fi.Close()

	return unpackArchive(distname, binnom, arcpath, out, archive)
}

func (i *Install) SelectGoodInstallLoc() error {
	var installDir string
	if i.StashedFromPath != "" {
		installDir = i.StashedFromPath
	} else {
		d, err := findGoodInstallDir()
		if err != nil {
			return err
		}

		installDir = d
	}

	i.InstallPath = filepath.Join(installDir, i.BinaryName)
	return nil
}

var errNoGoodInstall = fmt.Errorf("could not find good install location")

func findGoodInstallDir() (string, error) {
	// Gather some candidate locations
	// The first ones have more priority than the last ones
	var candidates []string

	// First candidate GOBIN
	gobin := os.Getenv("GOBIN")
	if gobin != "" {
		candidates = append(candidates, gobin)
	}

	// Second candidates, GOPATH(s)/bin
	gopath := os.Getenv("GOPATH")
	if gopath != "" {
		gopaths := strings.Split(gopath, ":")
		for i, _ := range gopaths {
			gopaths[i] = filepath.Join(gopaths[i], "bin")
		}
		candidates = append(candidates, gopaths...)
	}

	candidates = append(candidates, "/usr/local/bin")

	// Let's try user's $HOME/bin too
	// but not root because no one installs to /root/bin
	if home := os.Getenv("HOME"); home != "" && os.Getenv("USER") != "root" {
		homebin := filepath.Join(home, "bin")
		candidates = append(candidates, homebin)
	}

	// Finally /usr/bin
	candidates = append(candidates, "/usr/bin")
	// Test if it makes sense to install to any of those
	for _, dir := range candidates {
		if canWrite(dir) && isInPath(dir) {
			return dir, nil
		}
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
