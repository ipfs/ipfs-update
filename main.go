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

	cli "github.com/codegangsta/cli"
	stump "github.com/whyrusleeping/stump"
)

var gateway = "https://ipfs.io"

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

	stump.Log("stashing old binary")
	oldpath, err := StashOldBinary(currentVersion, false)
	if err != nil {
		return err
	}

	stump.Log("installing new binary to %s", oldpath)
	err = InstallBinaryTo(binpath, oldpath)
	if err != nil {
		// in case of error here, replace old binary
		stump.Error("Install failed: ", err)
		revertOldBinary(oldpath, currentVersion)
		return err
	}

	if beforeVersion("v0.3.10", v) {
		stump.VLog("  - ipfs pre v0.3.10 does not support checking of repo version through the tool")
		stump.VLog("  - if a migration is needed, you will be prompted when starting ipfs")
	} else {
		err := CheckMigration()
		if err != nil {
			stump.Error("Migration Failed: ", err)
			revertOldBinary(oldpath, currentVersion)
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
		stump.Verbose = c.Bool("verbose")
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:  "versions",
			Usage: "print out all available versions",
			Action: func(c *cli.Context) {
				vs, err := GetVersions(basehash)
				if err != nil {
					stump.Fatal("Failed to query versions: ", err)
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
					stump.Fatal("Failed to check local version: ", err)
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
					stump.Fatal("Please specify a version to install")
				}
				if vers == "latest" {
					latest, err := GetLatestVersion(basehash)
					if err != nil {
						stump.Fatal("error resolving 'latest': ", err)
					}
					vers = latest
				}

				err := InstallVersion(basehash, vers, c.Bool("no-check"))
				if err != nil {
					stump.Fatal(err)
				}
				stump.Log("\ninstallation complete.")

				if hasDaemonRunning() {
					stump.Log("remember to restart your daemon before continuing")
				}
			},
		},
		{
			Name:  "stash",
			Usage: "stashes copy of currently installed ipfs binary",
			Flags: []cli.Flag{
				cli.StringFlag{
					Name:  "tag",
					Usage: "optionally specify tag for stashed binary",
				},
			},
			Action: func(c *cli.Context) {
				tag := c.String("tag")
				if tag == "" {
					vers, err := GetCurrentVersion()
					if err != nil {
						stump.Fatal(err)
					}
					tag = vers
				}

				_, err := StashOldBinary(tag, true)
				if err != nil {
					stump.Fatal(err)
				}
			},
		},
		{
			Name:  "revert",
			Usage: "revert to previously installed version of ipfs",
			Description: `revert will check if a previous update left a stashed
binary and overwrite the current ipfs binary with it.`,
			Action: func(c *cli.Context) {
				oldbinpath, err := selectRevertBin()
				if err != nil {
					stump.Fatal(err)
				}

				oldpath, err := ioutil.ReadFile(filepath.Join(ipfsDir(), "old-bin", "path-old"))
				if err != nil {
					stump.Fatal("Path for previous installation could not be read: ", err)
				}

				binpath := string(oldpath)
				err = InstallBinaryTo(oldbinpath, binpath)
				if err != nil {
					stump.Error("failed to move old binary: %s", oldbinpath)
					stump.Error("to path: %s", binpath)
					stump.Fatal(err)
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
						stump.Fatal("error querying latest version: ", err)
					}

					vers = latest
				}

				output := "ipfs-" + vers
				ofl := c.String("output")
				if ofl != "" {
					output = ofl
				}

				_, err := os.Stat(output)
				if err == nil {
					stump.Fatal("file named %s already exists")
				}

				if !os.IsNotExist(err) {
					stump.Fatal("stat(%s)", output, err)
				}

				err = GetBinaryForVersion(basehash, vers, output)
				if err != nil {
					stump.Fatal("Failed to fetch binary: ", err)
				}

				err = os.Chmod(output, 0755)
				if err != nil {
					stump.Fatal("setting new binary executable: ", err)
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
