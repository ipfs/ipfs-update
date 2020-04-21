package lib

import (
	"errors"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/blang/semver"
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

	return filepath.Join(tmpd, util.OsExeFileName("ipfs-new")), nil
}

func NewInstall(root, target string, nocheck, downgrade bool) (*Install, error) {
	return &Install{
		TargetVers: target,
		UrlRoot:    root,
		NoCheck:    nocheck,
		Downgrade:  downgrade,
		BinaryName: util.OsExeFileName("ipfs"),
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

	NoCheck   bool
	Downgrade bool

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
	} else if i.CurrentVers == i.TargetVers {
		stump.Log("Already have version %s installed, skipping.", i.TargetVers)
		i.Succeeded = true
		return nil
	} else if !i.Downgrade {
		semverCurrent, err := semver.ParseTolerant(i.CurrentVers)
		if err != nil {
			return err
		}
		semverTarget, err := semver.ParseTolerant(i.TargetVers)
		if err != nil {
			return err
		}
		if semverTarget.LT(semverCurrent) {
			return errors.New("in order to downgrade, please pass the --allow-downgrade flag or use \"revert\"")
		}
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
				stump.Log(util.BoldText("this could be because you have a daemon running, but no ipfs binary in your path."))
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
	loc, err := exec.LookPath(util.OsExeFileName("ipfs"))
	if err != nil {
		return "", fmt.Errorf("could not find old binary: %s", err)
	}
	loc, err = filepath.Abs(loc)
	if err != nil {
		return "", fmt.Errorf("could not determine absolute path for old binary: %s", err)
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
		return "", fmt.Errorf("could not stash path: %s", err)
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
		return fmt.Errorf("failed to get ipfs binary: %s", err)
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

func goenv(env string) (string, error) {
	value, err := exec.Command("go", "env", env).Output()
	return strings.TrimRight(string(value), "\r\n"), err
}

func findGoodInstallDir() (string, error) {
	sysPath := filepath.SplitList(os.Getenv("PATH"))
	for i, s := range sysPath {
		sysPath[i] = filepath.Clean(s)
	}
	inPath := func(s string) bool {
		for _, p := range sysPath {
			if p == s {
				return true
			}
		}
		return false
	}

	// First, try the user's GOBIN directory. If it's configured and is in
	// the user's path, use it.
	gobin, err := goenv("GOBIN")
	if err == nil && len(gobin) > 0 {
		stump.Log("checking if we should install in GOBIN: %s", gobin)
		gobin := filepath.Clean(gobin)
		if inPath(gobin) && ensure(gobin) {
			return gobin, nil
		}
	}

	// Then, if the user has go installed and has setup a go environment
	// _AND_ has added it's bin directory to their path, prefer that.
	gopath, err := goenv("GOPATH")
	if err == nil {
		gopaths := filepath.SplitList(gopath)
		for _, path := range gopaths {
			gobin := filepath.Clean(filepath.Join(path, "bin"))
			stump.Log("checking if we should install in GOPATH: %s", gobin)
			if inPath(gobin) && ensure(gobin) {
				return gobin, nil
			}
		}
	}

	// If we're on windows, we don't have many options. Try the current
	// directory then try the directory with this binary.
	if runtime.GOOS == "windows" {
		stump.Log("checking known windows install locations")
		cwd, err := os.Getwd()
		if err == nil {
			cwd = filepath.Clean(cwd)
			if inPath(cwd) && canWrite(cwd) {
				return cwd, nil
			}
		}

		ep, err := os.Executable()
		if err == nil {
			dir := filepath.Clean(filepath.Dir(ep))
			if inPath(dir) && canWrite(dir) {
				return dir, nil
			}

			if cwd == dir && canWrite(dir) {
				_, exeName := filepath.Split(ep)
				// [2020.01.28] on Windows, the "command search sequence" includes the current directory
				// while not included in %PATH%, it should be rare that this branch is traversed on accident and is likely expected to succeed on this platform
				stump.Log("current working directory is not within %%PATH%% variable, but %q exists in cwd; using cwd as install target", exeName)
				return dir, nil
			}
		}
		return "", errNoGoodInstall
	}

	// If we're root, prefer /usr/local/bin and /usr/bin. Root usually installs _globally_.
	if os.Getuid() == 0 {
		stump.Log("checking root install locations")
		for _, path := range []string{"/usr/local/bin", "/usr/bin"} {
			if inPath(path) && canWrite(path) {
				return path, nil
			}
		}
	}

	// If we can get the user's home directory, try the two known locations.
	if homedir, err := os.UserHomeDir(); err == nil {
		stump.Log("checking user install locations")
		tryPaths := []string{
			filepath.Join(homedir, ".local", "bin"), // xdg
			filepath.Join(homedir, "bin"),           // old way
		}

		// Filter to paths that are in PATH
		userPaths := tryPaths[:0]
		for _, path := range tryPaths {
			if inPath(path) {
				userPaths = append(userPaths, path)
			}
		}

		// Try installing in the first path that exists.
		for _, path := range userPaths {
			if canWrite(path) {
				return path, nil
			}
		}

		// Try creating a path.
		for _, path := range userPaths {
			if ensure(path) {
				return path, nil
			}
		}
	}

	return "", errNoGoodInstall
}

func ensure(dir string) bool {
	err := os.MkdirAll(dir, os.ModePerm)
	if err != nil {
		return false
	}
	return canWrite(dir)
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

	fi.Close()
	_ = os.Remove(fi.Name())
	return true
}
