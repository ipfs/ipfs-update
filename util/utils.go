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
	config "github.com/ipfs/ipfs-update/config"
	stump "github.com/whyrusleeping/stump"
)

var (
	GlobalGatewayUrl = "https://ipfs.io"
	LocalApiUrl      = "http://localhost:5001"
	IpfsVersionPath  = "/ipns/dist.ipfs.io"
)

func init() {
	if dist := os.Getenv("IPFS_DIST_PATH"); dist != "" {
		IpfsVersionPath = dist
	}
}

const fetchSizeLimit = 1024 * 1024 * 512

func ApiEndpoint(ipfspath string) (string, error) {
	apifile := filepath.Join(ipfspath, "api")

	val, err := ioutil.ReadFile(apifile)
	if err != nil {
		return "", err
	}

	parts := strings.Split(string(val), "/")
	if len(parts) != 5 {
		return "", fmt.Errorf("incorrectly formatted api string: %q", string(val))
	}

	return parts[2] + ":" + parts[4], nil
}

func httpGet(url string) (*http.Response, error) {
	req, err := http.NewRequest("GET", url, nil)
	if err != nil {
		return nil, fmt.Errorf("http.NewRequest error: %s", err)
	}

	req.Header.Set("User-Agent", config.GetUserAgent())

	resp, err := http.DefaultClient.Do(req)
	if err != nil {
		return nil, fmt.Errorf("http.DefaultClient.Do error: %s", err)
	}

	return resp, nil
}

func httpFetch(url string) (io.ReadCloser, error) {
	stump.VLog("fetching url: %s", url)
	resp, err := httpGet(url)
	if err != nil {
		return nil, err
	}

	if resp.StatusCode >= 400 {
		stump.Error("fetching resource: %s", resp.Status)
		mes, err := ioutil.ReadAll(resp.Body)
		if err != nil {
			return nil, fmt.Errorf("error reading error body: %s", err)
		}

		return nil, fmt.Errorf("%s: %s", resp.Status, string(mes))
	}

	return newLimitReadCloser(resp.Body, fetchSizeLimit), nil
}

func Fetch(ipfspath string) (io.ReadCloser, error) {
	stump.VLog("  - fetching %q", ipfspath)
	ep, err := ApiEndpoint(IpfsDir())
	if err == nil {
		sh := api.NewShell(ep)
		if sh.IsUp() {
			stump.VLog("  - using local ipfs daemon for transfer")
			rc, err := sh.Cat(ipfspath)
			if err != nil {
				return nil, err
			}

			return newLimitReadCloser(rc, fetchSizeLimit), nil
		}
	}

	return httpFetch(GlobalGatewayUrl + ipfspath)
}

type limitReadCloser struct {
	io.Reader
	io.Closer
}

func newLimitReadCloser(rc io.ReadCloser, limit int64) io.ReadCloser {
	return limitReadCloser{
		Reader: io.LimitReader(rc, limit),
		Closer: rc,
	}
}

// This function is needed because os.Rename doesnt work across filesystem
// boundaries.
func CopyTo(src, dest string) error {
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
	cmd.Env = append(os.Environ(), "IPFS_PATH="+p)
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

func BoldText(s string) string {
	return fmt.Sprintf("\033[1m%s\033[0m")
}
