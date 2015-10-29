package main

import (
	"archive/zip"
	"bufio"
	"fmt"
	cli "github.com/codegangsta/cli"
	api "github.com/ipfs/go-ipfs-api"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path"
	"runtime"
)

var verbose bool

func Log(format string, args ...interface{}) {
	if format[len(format)-1] != '\n' {
		format += "\n"
	}
	fmt.Printf(format, args...)
}

func VLog(format string, args ...interface{}) {
	if verbose {
		Log(format, args...)
	}
}

var gateway = "https://ipfs.io"

func httpFetch(url string) (io.ReadCloser, error) {
	VLog("fetching url: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http.Get error: %s", err)
	}

	if resp.StatusCode >= 400 {
		Log("error fetching resource: %s", resp.Status)
		mes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading error body: %s", err)
		}

		return nil, fmt.Errorf("%s: %s", resp.Status, string(mes))
	}

	return resp.Body, nil
}

func Fetch(ipfspath string) (io.ReadCloser, error) {
	sh := api.NewShell("http://localhost:5001")
	if sh.IsUp() {
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

	trgt, err := os.Create(dest)
	if err != nil {
		return err
	}

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

func InstallVersion(root, v string) error {
	Log("installing ipfs version %s", v)
	tmpd, err := ioutil.TempDir("", "ipfs-update")
	if err != nil {
		return err
	}

	binpath := path.Join(tmpd, "ipfs-new")

	Log("fetching %s binary...", v)
	err = GetBinaryForVersion(root, v, binpath)
	if err != nil {
		return err
	}

	Log("binary downloaded, verifying...")

	err = TestBinary(binpath, v)
	if err != nil {
		return err
	}

	Log("verified! stashing old binary")

	oldpath, err := StashOldBinary()
	if err != nil {
		return err
	}

	Log("installing new binary to %s", oldpath)
	err = InstallBinaryTo(binpath, oldpath)
	if err != nil {
		// in case of error here, replace old binary
		stashpath := path.Join(ipfsDir(), "old-bin", "ipfs-old")
		rnerr := os.Rename(stashpath, oldpath)
		if rnerr != nil {
			fmt.Println("error replacing binary after install fail: ", rnerr)
			fmt.Println("sorry :(")
			fmt.Println("your old ipfs binary should still be located at: ", stashpath)
		}
		return err
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

func ipfsDir() string {
	def := path.Join(os.Getenv("HOME"), ".ipfs")

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

	olddir := path.Join(ipfsdir, "old-bin")
	npath := path.Join(olddir, "ipfs-old")
	pathpath := path.Join(olddir, "path-old")

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
	err = os.Rename(loc, npath)
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

	zippath := path.Join(dir, finame)
	fi, err := os.Create(zippath)
	if err != nil {
		return err
	}

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

	basehash := "/ipfs/QmXUGEDqzHbeGAwj4w72uA9ZJ6iYEaMyfQuioLSLJvcQY6"

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "print verbose output",
		},
	}

	app.Before = func(c *cli.Context) error {
		verbose = c.Bool("verbose")
		return nil
	}

	app.Commands = []cli.Command{
		{
			Name:  "versions",
			Usage: "print out all available versions",
			Action: func(c *cli.Context) {
				vs, err := GetVersions(basehash)
				if err != nil {
					fmt.Println("Failed to query versions: ", err)
					return
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
					fmt.Println("Failed to check local version: ", err)
					return
				}

				fmt.Println(v)
			},
		},
		{
			Name:  "install",
			Usage: "install a version of ipfs",
			Action: func(c *cli.Context) {
				vers := c.Args().First()
				if vers == "" {
					fmt.Println("Please specify a version to install")
					return
				}
				if vers == "latest" {
					latest, err := GetLatestVersion(basehash)
					if err != nil {
						fmt.Println("error resolving 'latest': ", err)
						return
					}
					vers = latest
				}

				err := InstallVersion(basehash, vers)
				if err != nil {
					fmt.Println(err)
					return
				}
			},
		},
		{
			Name:  "revert",
			Usage: "revert to previously installed version of ipfs",
			Description: `revert will check if a previous update left a stashed
binary and overwrite the current ipfs binary with it.`,
			Action: func(c *cli.Context) {
				oldbinpath := path.Join(ipfsDir(), "old-bin", "ipfs-old")
				_, err := os.Stat(oldbinpath)
				if os.IsNotExist(err) {
					fmt.Printf("No prior binary found at: %s\n", err)
					return
				}

				oldpath, err := ioutil.ReadFile(path.Join(ipfsDir(), "old-bin", "path-old"))
				if err != nil {
					fmt.Println("Path for previous installation could not be read: ", err)
					return
				}

				binpath := string(oldpath)
				err = InstallBinaryTo(oldbinpath, binpath)
				if err != nil {
					fmt.Printf("failed to move old binary: %s\n", oldbinpath)
					fmt.Printf("to path: %s\n%s\n", binpath, err)
					return
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
