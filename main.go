package main

import (
	"bufio"
	"context"
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"os/exec"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
	"github.com/ipfs/ipfs-update/config"
	"github.com/ipfs/ipfs-update/lib"
	"github.com/ipfs/ipfs-update/util"

	"github.com/urfave/cli/v2"
	"github.com/whyrusleeping/stump"
)

func init() {
	stump.ErrOut = os.Stderr
}

func main() {
	// HACK: [Windows compat] InsideGUI must be called before any text is printed to the console because that's how the WINAPI works (not my fault)
	if runtime.GOOS == "windows" {
		const windowsHelpURL = "https://youtu.be/UCQTSszdVmQ"
		if len(os.Args) == 1 && util.InsideGUI() {
			_, exeName := path.Split(os.Args[0])
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
		&cli.BoolFlag{
			Name:  "verbose",
			Usage: "Print verbose output.",
		},
		&cli.StringFlag{
			Name:  "distpath",
			Usage: "specify the distributions build to use",
		},
	}

	app.Before = func(c *cli.Context) error {
		stump.Verbose = c.Bool("verbose")
		return nil
	}

	app.Commands = []*cli.Command{
		cmdVersions,
		cmdVersion,
		cmdInstall,
		cmdStash,
		cmdRevert,
		cmdFetch,
	}

	ctx, cancel := context.WithCancel(context.Background())
	defer cancel()

	if err := app.RunContext(ctx, os.Args); err != nil {
		stump.Fatal(err)
	}
}

var cmdVersions = &cli.Command{
	Name:      "versions",
	Usage:     "Print out all available versions.",
	ArgsUsage: " ",
	Action: func(c *cli.Context) error {
		fetcher := createFetcher(c)
		vs, err := migrations.DistVersions(c.Context, fetcher, "go-ipfs", true)
		if err != nil {
			stump.Fatal("failed to query versions:", err)
		}

		for _, v := range vs {
			fmt.Println(v)
		}

		return nil
	},
}

var cmdVersion = &cli.Command{
	Name:  "version",
	Usage: "Print out currently installed version.",
	Action: func(c *cli.Context) error {
		v, err := lib.CurrentIpfsVersion()
		if err != nil {
			stump.Fatal("failed to check local version:", err)
		}

		fmt.Println(v)
		return nil
	},
}

var cmdInstall = &cli.Command{
	Name:      "install",
	Usage:     "Install a version of ipfs.",
	ArgsUsage: "A version or \"latest\" for the latest stable version or \"beta\" for the latest stable or RC version",
	Flags: []cli.Flag{
		&cli.BoolFlag{
			Name:  "no-check",
			Usage: "Skip pre-install tests.",
		},
		&cli.BoolFlag{
			Name:  "allow-downgrade",
			Usage: "Allow downgrading. WARNING: Downgrades may require running reverse migrations.",
		},
	},
	Action: func(c *cli.Context) error {
		vers := c.Args().First()
		if vers == "" {
			stump.Fatal("please specify a version to install")
		}

		fetcher := createFetcher(c)

		if vers == "latest" || vers == "beta" {
			stable := vers == "latest"
			latest, err := migrations.LatestDistVersion(c.Context, fetcher, "go-ipfs", stable)
			if err != nil {
				stump.Fatal("error resolving %q: %s", vers, err)
			}
			vers = latest
		}

		vers = checkVersionFormat(vers)

		i := lib.NewInstall(vers, c.Bool("no-check"), c.Bool("allow-downgrade"), fetcher)
		err := i.Run(c.Context)
		if err != nil {
			return fmt.Errorf("install failed: %s", err)
		}
		stump.Log("\nInstallation complete!")

		_, _, err = lib.ApiShell("")
		if err == nil {
			stump.Log("Remember to restart your daemon before continuing.")
		}

		return nil
	},
}

var cmdStash = &cli.Command{
	Name:  "stash",
	Usage: "stashes copy of currently installed ipfs binary",
	Description: `'stash' is an advanced command that saves the currently installed
   version of ipfs to a backup location. This is useful when you want to experiment
   with different versions, but still be able to go back to the version you started with.`,
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "tag",
			Usage: "Optionally specify tag for stashed binary.",
		},
	},
	Action: func(c *cli.Context) error {
		tag := c.String("tag")
		if tag == "" {
			vers, err := lib.CurrentIpfsVersion()
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

var cmdRevert = &cli.Command{
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

		stump.Log("Reverting to %s", oldbinpath)
		ipfsDir, err := migrations.IpfsDir("")
		if err != nil {
			stump.Fatal("could not find ipfs directory:", err)
		}
		oldpath, err := ioutil.ReadFile(path.Join(ipfsDir, "old-bin", "path-old"))
		if err != nil {
			stump.Fatal("path for previous installation could not be read:", err)
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

var cmdFetch = &cli.Command{
	Name:      "fetch",
	Usage:     "Fetch a given version of ipfs, or \"latest\" for the latest stable version or \"beta\" for the latest stable or RC version. Default: latest.",
	ArgsUsage: "<version>",
	Flags: []cli.Flag{
		&cli.StringFlag{
			Name:  "output",
			Usage: "Specify where to save the downloaded binary.",
		},
	},
	Action: func(c *cli.Context) error {
		fetcher := createFetcher(c)

		vers := c.Args().First()
		if vers == "" || vers == "latest" || vers == "beta" {
			var stable bool
			if vers == "beta" {
				stump.VLog("looking up 'beta'")
			} else {
				vers = "latest"
				stump.VLog("looking up 'latest'")
				stable = true
			}
			latest, err := migrations.LatestDistVersion(c.Context, fetcher, "go-ipfs", stable)
			if err != nil {
				stump.Fatal("error querying %q version: %s", vers, err)
			}

			vers = latest
		}

		vers = checkVersionFormat(vers)

		var err error
		output := c.String("output")
		if output == "" {
			output = migrations.ExeName("ipfs-" + vers)
		}

		stump.Log("fetching go-ipfs version", vers)

		_, err = migrations.FetchBinary(c.Context, fetcher, "go-ipfs", vers, "ipfs", output)
		if err != nil {
			stump.Fatal("failed to fetch binary:", err)
		}

		return nil
	},
}

func checkVersionFormat(ver string) string {
	if !strings.HasPrefix(ver, "v") && looksLikeSemver(ver) {
		stump.VLog("Version strings must start with 'v'. Autocorrecting...")
		ver = "v" + ver
	}
	return ver
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

func createFetcher(c *cli.Context) migrations.Fetcher {
	const userAgent = "ipfs-update"

	distPath := c.String("distpath")
	if distPath == "" {
		distPath = migrations.GetDistPathEnv("")
	}

	customIpfsGatewayURL := os.Getenv("IPFS_GATEWAY") // uses https://ipfs.io as default, if unset

	return migrations.NewMultiFetcher(
		lib.NewIpfsFetcher(distPath, 0),
		&retryFetcher{
			Fetcher:    migrations.NewHttpFetcher(distPath, customIpfsGatewayURL, userAgent, 0),
			maxRetries: 3,
		})
}

type retryFetcher struct {
	migrations.Fetcher
	maxRetries int
}

var _ migrations.Fetcher = (*retryFetcher)(nil)

func (r *retryFetcher) Fetch(ctx context.Context, filePath string) (io.ReadCloser, error) {
	var lastErr error
	for i := 0; i < r.maxRetries; i++ {
		out, err := r.Fetcher.Fetch(ctx, filePath)
		if err == nil {
			return out, nil
		}
		lastErr = err
	}
	return nil, fmt.Errorf("exceeded number of retries. last error was %w", lastErr)
}

func (r *retryFetcher) Close() error {
	return r.Fetcher.Close()
}
