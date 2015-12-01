package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"net/http"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	api "github.com/ipfs/go-ipfs-api"
	stump "github.com/whyrusleeping/stump"
)

var (
	GlobalGatewayUrl = "https://ipfs.io"
	LocalApiUrl      = "http://localhost:5001"
	IpfsVersionPath  = "/ipns/update.ipfs.io"
)

func httpFetch(url string) (io.ReadCloser, error) {
	stump.VLog("fetching url: %s", url)
	resp, err := http.Get(url)
	if err != nil {
		return nil, fmt.Errorf("http.Get error: %s", err)
	}

	if resp.StatusCode >= 400 {
		stump.Error("fetching resource: %s", resp.Status)
		mes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading error body: %s", err)
		}

		return nil, fmt.Errorf("%s: %s", resp.Status, string(mes))
	}

	return resp.Body, nil
}

func Fetch(ipfspath string) (io.ReadCloser, error) {
	stump.VLog("  - fetching %q", ipfspath)
	sh := api.NewShell(LocalApiUrl)
	if sh.IsUp() {
		stump.VLog("  - using local ipfs daemon for transfer")
		return sh.Cat(ipfspath)
	}

	return httpFetch(GlobalGatewayUrl + ipfspath)
}

// This function is needed because os.Rename doesnt work across filesystem
// boundaries.
func CopyTo(src, dest string) error {
	stump.VLog("  - copying %s to %s", src, dest)
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

func Move(src, dest string) error {
	err := CopyTo(src, dest)
	if err != nil {
		return err
	}

	return os.Remove(src)
}

func IpfsDir() string {
	def := filepath.Join(os.Getenv("HOME"), ".ipfs")

	ipfs_path := os.Getenv("IPFS_PATH")
	if ipfs_path != "" {
		def = ipfs_path
	}

	return def
}

func HasDaemonRunning() bool {
	shell := api.NewShell(LocalApiUrl)
	return shell.IsUp()
}

func RunCmd(p, bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	cmd.Env = []string{"IPFS_PATH=" + p}
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(out))
	}

	if out[len(out)-1] == '\n' {
		return string(out[:len(out)-1]), nil
	}
	return string(out), nil
}

func BeforeVersion(check, cur string) bool {
	aparts := strings.Split(check[1:], ".")
	bparts := strings.Split(cur[1:], ".")
	for i := 0; i < 3; i++ {
		an, err := strconv.Atoi(aparts[i])
		if err != nil {
			return false
		}
		bn, err := strconv.Atoi(bparts[i])
		if err != nil {
			return false
		}
		if bn < an {
			return true
		}
		if bn > an {
			return false
		}
	}
	return false
}
