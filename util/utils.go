package util

import (
	"fmt"
	"io"
	"io/ioutil"
	"os"
	"path"
	"runtime"
	"strconv"
	"strings"

	"github.com/ipfs/go-ipfs/repo/fsrepo/migrations"
)

// Local IPFS API
const apiFile = "api"

var (
	// forceRemove tries to remove a file, even if it's in-use. On windows,
	// this function will move open files to a temporary directory.
	forceRemove = func(filePath string) error {
		return os.Remove(filePath)
	}

	InsideGUI = func() bool { return false }
)

// [2018.06.06] This function is needed because os.Rename doesn't work across filesystem
// boundaries.
func CopyTo(src, dest string) error {
	fi, err := os.Open(src)
	if err != nil {
		return err
	}
	defer fi.Close()

	if runtime.GOOS == "windows" {
		// On windows, we need to remove this file first if it's in-use
		// (i.e., IPFS is running).
		if err := forceRemove(dest); err != nil {
			return fmt.Errorf("copy dest exists and can not be deleted: %s", err)
		}
	}

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

	return forceRemove(src)
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

// ApiEndpoint reads the api file from the local ipfs install directory and
// returns the address:port read from the file.  If the ipfs directory is not
// specified then the default location is used.
func ApiEndpoint(ipfsDir string) (string, error) {
	ipfsDir, err := migrations.CheckIpfsDir(ipfsDir)
	if err != nil {
		return "", err
	}

	apiData, err := ioutil.ReadFile(path.Join(ipfsDir, apiFile))
	if err != nil {
		return "", err
	}

	val := strings.TrimSpace(string(apiData))
	parts := strings.Split(val, "/")
	if len(parts) != 5 {
		return "", fmt.Errorf("incorrectly formatted api string: %q", val)
	}

	return parts[2] + ":" + parts[4], nil
}
