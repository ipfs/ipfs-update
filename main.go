package main

import (
	"bufio"
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strconv"
	"strings"

	config "github.com/ipfs/ipfs-update/config"
	lib "github.com/ipfs/ipfs-update/lib"
	util "github.com/ipfs/ipfs-update/util"

	cli "github.com/urfave/cli"
	stump "github.com/whyrusleeping/stump"
)

func init() {
	stump.ErrOut = os.Stderr
}

func main() {
	// HACK: [Windows compat] InsideGUI must be called before any text is printed to the console because that's how the WINAPI works (not my fault)
	if runtime.GOOS == "windows" {
		const windowsHelpURL = "https://youtu.be/UCQTSszdVmQ"
		if len(os.Args) == 1 && util.InsideGUI() {
			_, exeName := filepath.Split(os.Args[0])
			stump.Log(`%q is a command line application.
If you would like to open a video demonstrating how to use it, press return/enter.
(Will open browser with %q)
Otherwise you can close this window.`, exeName, windowsHelpURL)
			bufio.NewReader(os.Stdin).ReadBytes('\n')
			if err := exec.Command("rundll32", "url.dll,FileProtocolHandler", windowsHelpURL).Start(); err != nil {
				stump.Error("failed to launch browser: %s", err)
				bufio.NewReader(os.Stdin).ReadBytes('\n')
				return
			}
		}
	}

	app := cli.NewApp()
	app.Usage = "Update ipfs."
	app.Version = config.CurrentVersionNumber

	app.Flags = []cli.Flag{
		cli.BoolFlag{
			Name:  "verbose",
			Usage: "Print verbose output.",
		},
		cli.StringFlag{
			Name:  "distpath",
			Usage: "specify the distributions build to use",
		},
	}

	app.Before = func(c *cli.Context) error {
		stump.Verbose = c.Bool("verbose")
		if distp := c.String("distpath"); distp != "" {
			util.IpfsVersionPath = distp
		}
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

	if err := app.Run(os.Args); err != nil {
		stump.Fatal(err)
	}
}

var cmdVersions = cli.Command{
	Name:      "versions",
	Usage:     "Print out all available versions.",
	ArgsUsage: " ",
	Action: func(c *cli.Context) error {
		vs, err := lib.GetVersions(util.IpfsVersionPath, "go-ipfs")
		if err != nil {
			stump.Fatal("failed to query versions: ", err)
		}

		for _, v := range vs {
			fmt.Println(v)
		}

		return nil
	},
}

var cmdVersion = cli.Command{
	Name:  "version",
	Usage: "Print out currently installed version.",
	Action: func(c *cli.Context) error {
		v, err := lib.GetCurrentVersion()
		if err != nil {
			stump.Fatal("failed to check local version: ", err)
		}

		fmt.Println(v)
		return nil
	},
}

var cmdInstall = cli.Command{
	Name:      "install",
	Usage:     "Install a version of ipfs.",
	ArgsUsage: "A version or \"latest\" for latest version",
	Flags: []cli.Flag{
		cli.BoolFlag{
			Name:  "no-check",
			Usage: "Skip pre-install tests.",
		},
		cli.BoolFlag{
			Name:  "allow-downgrade",
			Usage: "Allow downgrading. WARNING: Downgrades may require running reverse migrations.",
		},
	},
	Action: func(c *cli.Context) error {
		vers := c.Args().First()
		if vers == "" {
			stump.Fatal("please specify a version to install")
		}
		if vers == "latest" {
			latest, err := lib.GetLatestVersion(util.IpfsVersionPath, "go-ipfs")
			if err != nil {
				stump.Fatal("error resolving 'latest': ", err)
			}
			vers = latest
		}

		if !strings.HasPrefix(vers, "v") && looksLikeSemver(vers) {
			stump.VLog("Version strings must start with 'v'. Autocorrecting...")
			vers = "v" + vers
		}

		i, err := lib.NewInstall(
			util.IpfsVersionPath,
			vers,
			c.Bool("no-check"),
			c.Bool("allow-downgrade"),
		)
		if err != nil {
			return err
		}

		err = i.Run()
		if err != nil {
			return err
		}
		stump.Log("\nInstallation complete!")

		if util.HasDaemonRunning() {
			stump.Log("Remember to restart your daemon before continuing.")
		}

		return nil
	},
}

var cmdStash = cli.Command{
	Name:  "stash",
	Usage: "stashes copy of currently installed ipfs binary",
	Description: `'stash' is an advanced command that saves the currently installed
   version of ipfs to a backup location. This is useful when you want to experiment
   with different versions, but still be able to go back to the version you started with.`,
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "tag",
			Usage: "Optionally specify tag for stashed binary.",
		},
	},
	Action: func(c *cli.Context) error {
		tag := c.String("tag")
		if tag == "" {
			vers, err := lib.GetCurrentVersion()
			if err != nil {
				return err
			}
			tag = vers
		}

		_, err := lib.StashOldBinary(tag, true)
		if err != nil {
			return err
		}

		return nil
	},
}

var cmdRevert = cli.Command{
	Name:      "revert",
	Usage:     "Revert to previously installed version of ipfs.",
	ArgsUsage: " ",
	Description: `'revert' will check if a previous update left a stashed
   binary and overwrite the current ipfs binary with it.

   Using 'revert' will not run any datastore migrations. For that, use
   'ipfs-update install --allow-downgrade <prev-version>'.

   If multiple previous versions exist, you will be prompted to select the
   desired binary.
`,
	Action: func(c *cli.Context) error {
		oldbinpath, err := lib.SelectRevertBin()
		if err != nil {
			return err
		}

		stump.Log("Reverting to %s.", oldbinpath)
		oldpath, err := ioutil.ReadFile(filepath.Join(util.IpfsDir(), "old-bin", "path-old"))
		if err != nil {
			stump.Fatal("path for previous installation could not be read: ", err)
		}

		binpath := string(oldpath)
		err = lib.InstallBinaryTo(oldbinpath, binpath)
		if err != nil {
			stump.Error("failed to move old binary: %s", oldbinpath)
			stump.Error("to path: %s", binpath)
			stump.Fatal(err)
		}
		stump.Log("\nRevert complete.")
		return nil
	},
}

var cmdFetch = cli.Command{
	Name:      "fetch",
	Usage:     "Fetch a given version of ipfs. Default: latest.",
	ArgsUsage: "<version>",
	Flags: []cli.Flag{
		cli.StringFlag{
			Name:  "output",
			Usage: "Specify where to save the downloaded binary.",
		},
	},
	Action: func(c *cli.Context) error {
		vers := c.Args().First()
		if vers == "" || vers == "latest" {
			stump.VLog("looking up 'latest'")
			latest, err := lib.GetLatestVersion(util.IpfsVersionPath, "go-ipfs")
			if err != nil {
				stump.Fatal("error querying latest version: ", err)
			}

			vers = latest
		}

		if !strings.HasPrefix(vers, "v") && looksLikeSemver(vers) {
			stump.VLog("Version strings must start with 'v'. Autocorrecting...")
			vers = "v" + vers
		}

		output := util.OsExeFileName("ipfs-" + vers)

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

		err = lib.GetBinaryForVersion("go-ipfs", "ipfs", util.IpfsVersionPath, vers, output)
		if err != nil {
			stump.Fatal("failed to fetch binary: ", err)
		}

		err = os.Chmod(output, 0755)
		if err != nil {
			stump.Fatal("setting new binary executable: ", err)
		}
		return nil
	},
}

func looksLikeSemver(v string) bool {
	parts := strings.Split(v, ".")
	if len(parts) < 3 {
		return false
	}

	if _, err := strconv.Atoi(parts[0]); err == nil {
		return true
	}

	return false
}
