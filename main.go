package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	cli "github.com/codegangsta/cli"
	api "github.com/ipfs/go-ipfs-api"
	. "github.com/whyrusleeping/stump"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
)

var gateway = "https://ipfs.io"

func httpFetch(url string) (io.ReadCloser, error) {
	VLog("fetching url: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http.Get error: %s", err)
	}

	if resp.StatusCode >= 400 {
		Error("fetching resource: %s", resp.Status)
		mes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading error body: %s", err)
		}

		return nil, fmt.Errorf("%s: %s", resp.Status, string(mes))
	}

	return resp.Body, nil
}

func Fetch(ipfspath string) (io.ReadCloser, error) {
	VLog("  - fetching %q", ipfspath)
	sh := api.NewShell("http://localhost:5001")
	if sh.IsUp() {
		VLog("  - using local ipfs daemon for transfer")
		return sh.Cat(ipfspath)
	}

	return httpFetch(gateway + ipfspath)
}

// This function is needed because os.Rename doesnt work across filesystem
// boundaries.
func CopyTo(src, dest string) error {
	VLog("  - copying %s to %s", src, dest)
	fi, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fi.Close()

	trgt, err := os.Create(dest)
	if err != nil {
		return err
	}
	defer trgt.Close()

	_, err = io.Copy(trgt, fi)
	return err
}

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

func InstallVersion(root, v string, nocheck bool) error {
	Log("installing ipfs version %s", v)
	tmpd, err := ioutil.TempDir("", "ipfs-update")
	if err != nil {
		return err
	}

	binpath := filepath.Join(tmpd, "ipfs-new")

	Log("fetching %s binary...", v)
	err = GetBinaryForVersion(root, v, binpath)
	if err != nil {
		return err
	}

	if !nocheck {
		Log("binary downloaded, verifying...")
		err = TestBinary(binpath, v)
		if err != nil {
			return err
		}
	} else {
		Log("skipping tests since '--no-check' was passed")
	}

	Log("stashing old binary")
	oldpath, err := StashOldBinary()
	if err != nil {
		return err
	}

	Log("installing new binary to %s", oldpath)
	err = InstallBinaryTo(binpath, oldpath)
	if err != nil {
		// in case of error here, replace old binary
		Error("Install failed: ", err)
		revertOldBinary(oldpath)
		return err
	}

	if beforeVersion("v0.3.10", v) {
		VLog("  - ipfs pre v0.3.10 does not support checking of repo version through the tool")
		VLog("  - if a migration is needed, you will be prompted when starting ipfs")
	} else {
		err := CheckMigration()
		if err != nil {
			Error("Migration Failed: ", err)
			revertOldBinary(oldpath)
			return err
		}
	}

	return nil
}

func CheckMigration() error {
	Log("checking if repo migration is needed...")
	p := ipfsDir()
	oldverB, err := ioutil.ReadFile(filepath.Join(p, "version"))
	if err != nil {
		return err
	}

	oldver := strings.Trim(string(oldverB), "\n \t")
	VLog("  - old repo version is", oldver)

	nbinver, err := runCmd("", "ipfs", "version", "--repo")
	if err != nil {
		Log("Failed to check new binary repo version.")
		VLog("Reason: ", err)
		Log("This is not an error.")
		Log("This just means that you may have to manually run the migration")
		Log("You will be prompted to do so upon starting the ipfs daemon if necessary")
		return nil
	}

	VLog("  - repo version of new binary is ", nbinver)

	if oldver != nbinver {
		Log("  - Migration required")
		return RunMigration(oldver, nbinver)
	}

	VLog("  - no migration required")

	return nil
}

func RunMigration(oldv, newv string) error {
	migrateBin := "fs-repo-migrations"
	_, err := exec.LookPath(migrateBin)
	if err != nil {
		return fmt.Errorf("could not locate fs-repo-migrations binary")
	}

	cmd := exec.Command(migrateBin, "-to", newv, "-y")

	cmd.Stdout = os.Stdout
	cmd.Stderr = os.Stderr

	Log("running migration: '%s -to %s -y'", migrateBin, newv)

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("migration failed: %s", err)
	}

	Log("migration succeeded!")
	return nil
}

func Move(src, dest string) error {
	err := CopyTo(src, dest)
	if err != nil {
		return err
	}

	return os.Remove(src)
}

func revertOldBinary(oldpath string) {
	stashpath := filepath.Join(ipfsDir(), "old-bin", "ipfs-old")
	rnerr := Move(stashpath, oldpath)
	if rnerr != nil {
		Log("error replacing binary after install fail: ", rnerr)
		Log("sorry :(")
		Log("your old ipfs binary should still be located at: ", stashpath)
	}
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

func ipfsDir() string {
	def := filepath.Join(os.Getenv("HOME"), ".ipfs")

	ipfs_path := os.Getenv("IPFS_PATH")
	if ipfs_path != "" {
		def = ipfs_path
	}

	return def
}

// StashOldBinary moves the existing ipfs binary to a backup directory
// and returns the path to the original location of the old binary
func StashOldBinary() (string, error) {
	loc, err := exec.LookPath("ipfs")
	if err != nil {
		return "", fmt.Errorf("could not find old binary: %s", err)
	}

	ipfsdir := ipfsDir()

	olddir := filepath.Join(ipfsdir, "old-bin")
	npath := filepath.Join(olddir, "ipfs-old")
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

	VLog("  - moving %s to %s", loc, npath)
	err = Move(loc, npath)
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

	VLog("  - writing to", zippath)
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

	VLog("  - extracting binary to tempdir: ", target)
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

func main() {
	app := cli.NewApp()
	app.Author = "whyrusleeping"
	app.Usage = "update ipfs"
	app.Version = "0.1.0"

	basehash := "/ipfs/QmSiTko9JZyabH56y2fussEt1A5oDqsFXB3CkvAqraFryz"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "print verbose output",
		},
	}

	app.Before = func(c *cli.Context) error {
		Verbose = c.Bool("verbose")
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:  "versions",
			Usage: "print out all available versions",
			Action: func(c *cli.Context) {
				vs, err := GetVersions(basehash)
				if err != nil {
					Fatal("Failed to query versions: ", err)
				}

				for _, v := range vs {
					fmt.Println(v)
				}
			},
		},
		{
			Name:  "version",
			Usage: "print out currently installed version",
			Action: func(c *cli.Context) {
				v, err := GetCurrentVersion()
				if err != nil {
					Fatal("Failed to check local version: ", err)
				}

				fmt.Println(v)
			},
		},
		{
			Name:  "install",
			Usage: "install a version of ipfs",
			Flags: []cli.Flag{
				cli.BoolFlag{
					Name:  "no-check",
					Usage: "skip running of pre-install tests",
				},
			},
			Action: func(c *cli.Context) {
				vers := c.Args().First()
				if vers == "" {
					Fatal("Please specify a version to install")
				}
				if vers == "latest" {
					latest, err := GetLatestVersion(basehash)
					if err != nil {
						Fatal("error resolving 'latest': ", err)
					}
					vers = latest
				}

				err := InstallVersion(basehash, vers, c.Bool("no-check"))
				if err != nil {
					Fatal(err)
				}
				Log("\ninstallation complete.")
			},
		},
		{
			Name:  "revert",
			Usage: "revert to previously installed version of ipfs",
			Description: `revert will check if a previous update left a stashed
binary and overwrite the current ipfs binary with it.`,
			Action: func(c *cli.Context) {
				oldbinpath := filepath.Join(ipfsDir(), "old-bin", "ipfs-old")
				_, err := os.Stat(oldbinpath)
				if os.IsNotExist(err) {
					Fatal("No prior binary found at:", err)
				}

				oldpath, err := ioutil.ReadFile(filepath.Join(ipfsDir(), "old-bin", "path-old"))
				if err != nil {
					Fatal("Path for previous installation could not be read: ", err)
				}

				binpath := string(oldpath)
				err = InstallBinaryTo(oldbinpath, binpath)
				if err != nil {
					Error("failed to move old binary: %s", oldbinpath)
					Error("to path: %s", binpath)
					Fatal(err)
				}
			},
		},
		{
			Name:  "fetch",
			Usage: "fetch a given (default: latest) version of ipfs",
			Action: func(c *cli.Context) {
				vers := c.Args().First()
				if vers == "" || vers == "latest" {
					latest, err := GetLatestVersion(basehash)
					if err != nil {
						fmt.Println("error querying latest version: ", err)
						return
					}

					vers = latest
				}

				output := "ipfs-" + vers
				ofl := c.String("output")
				if ofl != "" {
					output = ofl
				}

				err := GetBinaryForVersion(basehash, vers, output)
				if err != nil {
					fmt.Println("Failed to fetch binary: ", err)
					return
				}
			},
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "output",
					Usage: "specify where to save the downloaded binary",
				},
			},
		},
	}

	app.Run(os.Args)
}
