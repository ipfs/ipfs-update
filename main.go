package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strings"

	cli "github.com/codegangsta/cli"
	util "github.com/ipfs/ipfs-update/util"
	stump "github.com/whyrusleeping/stump"
)

func init() {
	stump.ErrOut = os.Stderr
}

func main() {
	app := cli.NewApp()
	app.Usage = "update ipfs"
	app.Version = "0.1.0"

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
		cmdVersions,
		cmdVersion,
		cmdInstall,
		cmdStash,
		cmdRevert,
		cmdFetch,
	}

	app.Run(os.Args)
}

var cmdVersions = cli.Command{
	Name:      "versions",
	Usage:     "print out all available versions",
	ArgsUsage: " ",
	Action: func(c *cli.Context) {
		vs, err := GetVersions(util.IpfsVersionPath)
		if err != nil {
			stump.Fatal("Failed to query versions: ", err)
		}

		for _, v := range vs {
			fmt.Println(v)
		}
	},
}

var cmdVersion = cli.Command{
	Name:  "version",
	Usage: "print out currently installed version",
	Action: func(c *cli.Context) {
		v, err := GetCurrentVersion()
		if err != nil {
			stump.Fatal("Failed to check local version: ", err)
		}

		fmt.Println(v)
	},
}

var cmdInstall = cli.Command{
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
			latest, err := GetLatestVersion(util.IpfsVersionPath)
			if err != nil {
				stump.Fatal("error resolving 'latest': ", err)
			}
			vers = latest
		}
		if !strings.HasPrefix(vers, "v") {
			stump.VLog("version strings must start with 'v', autocorrecting...")
			vers = "v" + vers
		}

		i, err := NewInstall(util.IpfsVersionPath, vers, c.Bool("no-check"))
		if err != nil {
			stump.Fatal(err)
		}

		err = i.Run()
		if err != nil {
			stump.Fatal(err)
		}
		stump.Log("\ninstallation complete.")

		if util.HasDaemonRunning() {
			stump.Log("remember to restart your daemon before continuing")
		}
	},
}

var cmdStash = cli.Command{
	Name:  "stash",
	Usage: "stashes copy of currently installed ipfs binary",
	Description: `stash is an advanced command that saves the currently installed
   version of ipfs to a backup location. This is useful when you want to experiment
   with different versions but still be able to go back to the version you started from.`,
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
}

var cmdRevert = cli.Command{
	Name:      "revert",
	Usage:     "revert to previously installed version of ipfs",
	ArgsUsage: " ",
	Description: `revert will check if a previous update left a stashed
   binary and overwrite the current ipfs binary with it.
   if multiple previous versions exist, you will be prompted to select the
   desired binary.`,
	Action: func(c *cli.Context) {
		oldbinpath, err := selectRevertBin()
		if err != nil {
			stump.Fatal(err)
		}

		stump.Log("reverting to %s", oldbinpath)
		oldpath, err := ioutil.ReadFile(filepath.Join(util.IpfsDir(), "old-bin", "path-old"))
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
		stump.Log("\nrevert complete.")
	},
}

var cmdFetch = cli.Command{
	Name:      "fetch",
	Usage:     "fetch a given (default: latest) version of ipfs",
	ArgsUsage: "<version>",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output",
			Usage: "specify where to save the downloaded binary",
		},
	},
	Action: func(c *cli.Context) {
		vers := c.Args().First()
		if vers == "" || vers == "latest" {
			stump.VLog("looking up 'latest'")
			latest, err := GetLatestVersion(util.IpfsVersionPath)
			if err != nil {
				stump.Fatal("error querying latest version: ", err)
			}

			vers = latest
		}

		if !strings.HasPrefix(vers, "v") {
			stump.VLog("version strings must start with 'v', autocorrecting...")
			vers = "v" + vers
		}

		output := "ipfs-" + vers
		ofl := c.String("output")
		if ofl != "" {
			output = ofl
		}

		_, err := os.Stat(output)
		if err == nil {
			stump.Fatal("file named %q already exists", output)
		}

		if !os.IsNotExist(err) {
			stump.Fatal("stat(%s)", output, err)
		}

		err = GetBinaryForVersion(util.IpfsVersionPath, vers, output)
		if err != nil {
			stump.Fatal("Failed to fetch binary: ", err)
		}

		err = os.Chmod(output, 0755)
		if err != nil {
			stump.Fatal("setting new binary executable: ", err)
		}
	},
}
