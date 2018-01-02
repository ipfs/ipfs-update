package testdist

import (
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path/filepath"
	"runtime"
	"strings"
	"time"

	util "github.com/ipfs/ipfs-update/util"
	stump "github.com/whyrusleeping/stump"
)

func runCmd(p, bin string, args ...string) (string, error) {
	cmd := exec.Command(bin, args...)
	if runtime.GOOS == "windows" {
		cmd.Env = os.Environ()
	}
	cmd.Env = util.ReplaceEnvVarIfExists(cmd.Env, "IPFS_PATH", p)
	stump.VLog("  - running: %s", cmd.Args)
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s: %s", err, string(out))
	}

	return strings.TrimSpace(string(out)), nil
}

type daemon struct {
	p      *os.Process
	stderr io.WriteCloser
	stdout io.WriteCloser
}

func (d *daemon) Close() error {
	err := d.p.Kill()
	if err != nil {
		stump.Error("error killing daemon: %s", err)
		return err
	}

	_, err = d.p.Wait()
	if err != nil {
		stump.Error("error waiting on killed daemon: %s", err)
		return err
	}

	d.stderr.Close()
	d.stdout.Close()
	return nil
}

func tweakConfig(ipfspath string) error {
	cfgpath := filepath.Join(ipfspath, "config")
	cfg := make(map[string]interface{})
	cfgbytes, err := ioutil.ReadFile(cfgpath)
	if err != nil {
		return err
	}

	err = json.Unmarshal(cfgbytes, &cfg)
	if err != nil {
		return err
	}

	cfg["Discovery"].(map[string]interface{})["MDNS"].(map[string]interface{})["Enabled"] = false

	addrs, ok := cfg["Addresses"].(map[string]interface{})
	if !ok {
		return fmt.Errorf("no addresses field in config")
	}

	addrs["API"] = "/ip4/127.0.0.1/tcp/0"
	addrs["Gateway"] = ""
	addrs["Swarm"] = []string{"/ip4/0.0.0.0/tcp/0"}

	_, ok = cfg["Bootstrap"].([]interface{})
	if !ok {
		fmt.Println(cfg["Bootstrap"])
		return fmt.Errorf("no bootstrap field in config")
	}
	cfg["Bootstrap"] = []interface{}{}

	out, err := json.Marshal(cfg)
	if err != nil {
		return err
	}

	err = ioutil.WriteFile(cfgpath, out, 0644)
	if err != nil {
		return fmt.Errorf("error writing tweaked config: %s", err)
	}

	return nil
}

func StartDaemon(p, bin string) (io.Closer, error) {
	cmd := exec.Command(bin, "daemon", "--debug")

	stdout, err := os.Create(filepath.Join(p, "daemon.stdout"))
	if err != nil {
		return nil, err
	}

	stderr, err := os.Create(filepath.Join(p, "daemon.stderr"))
	if err != nil {
		return nil, err
	}

	if runtime.GOOS == "windows" {
		cmd.Env = os.Environ()
	}
	cmd.Env = util.ReplaceEnvVarIfExists(cmd.Env, "IPFS_PATH", p)
	cmd.Stdout = stdout
	cmd.Stderr = stderr

	err = cmd.Start()
	if err != nil {
		stdout.Close()
		stderr.Close()
		return nil, fmt.Errorf("failed to start daemon: %s", err)
	}

	// now wait for api to become live
	err = waitForApi(p)
	if err != nil {
		return nil, err
	}

	return &daemon{
		p:      cmd.Process,
		stderr: stderr,
		stdout: stdout,
	}, nil
}

func waitForApi(ipfspath string) error {
	stump.VLog("  - waiting on daemon to come online")
	var endpoint string
	nloops := 15
	var success bool
	for i := 0; i < nloops; i++ {
		ep, err := util.ApiEndpoint(ipfspath)
		if err == nil {
			stump.VLog("  - found api file: %s", ep)
			endpoint = ep
			success = true
			break
		}
		if !os.IsNotExist(err) {
			return err
		}

		time.Sleep(time.Millisecond * (100 * time.Duration(i+1)))
	}

	if !success {
		stump.VLog("  - no api file found, trying fallback (happens pre 0.3.8)")
		endpoint = "localhost:5001"
	}

	for i := 0; i < 10; i++ {
		c, err := net.Dial("tcp", endpoint)
		if err == nil {
			c.Close()
			stump.VLog("  - Successfully made connection to api endpoint")
			return nil
		}
		stump.VLog("  - connecting to api endpoint failed: %s", err)

		time.Sleep(time.Millisecond * (100 * time.Duration(i+1)))
	}

	return fmt.Errorf("failed to come online")
}

func TestBinary(bin, version string) error {
	_, err := os.Stat(bin)
	if err != nil {
		return err
	}

	// make sure binary is executable
	err = os.Chmod(bin, 0755)
	if err != nil {
		return err
	}

	staging := filepath.Join(util.IpfsDir(), "update-staging")
	err = os.MkdirAll(staging, 0755)
	if err != nil {
		return fmt.Errorf("error creating test staging directory: %s", err)
	}

	tdir, err := ioutil.TempDir(staging, "test")
	if err != nil {
		return err
	}

	err = os.MkdirAll(tdir, 0755)
	if err != nil {
		return fmt.Errorf("error creating test staging directory: %s", err)
	}

	defer func(dir string) {
		// defer cleanup, bound param to avoid mistakes
		err = os.RemoveAll(dir)
		if err != nil {
			stump.Error("error cleaning up staging directory: ", err)
		}
	}(tdir)

	stump.VLog("  - running init in '%s' with new binary", tdir)
	_, err = runCmd(tdir, bin, "init")
	if err != nil {
		return fmt.Errorf("error initializing with new binary: %s", err)
	}

	stump.VLog("  - checking new binary outputs correct version")
	rversion, err := runCmd(tdir, bin, "version")
	if err != nil {
		return err
	}

	parts := strings.Fields(rversion)
	if !versionMatch(parts[len(parts)-1], version[1:]) {
		return fmt.Errorf("version didnt match (expected '%s', got '%s')", version[1:], parts[len(parts)-1])
	}

	if util.BeforeVersion("v0.3.8", version) {
		stump.Log("== skipping tests with daemon, versions before 0.3.8 do not support port zero ==")
		return nil
	}

	// set up ports in config so we dont interfere with an already running daemon
	stump.VLog("  - tweaking test config to avoid external interference")
	err = tweakConfig(tdir)
	if err != nil {
		return err
	}

	stump.VLog("  - starting up daemon")
	daemon, err := StartDaemon(tdir, bin)
	if err != nil {
		return fmt.Errorf("error starting daemon: %s", err)
	}
	defer func() {
		stump.VLog("  - killing test daemon")
		err := daemon.Close()
		if err != nil {
			stump.VLog("  - error killing test daemon: %s (continuing anyway)", err)
		}
	}()

	// test some basic things against the daemon
	err = testFileAdd(tdir, bin)
	if err != nil {
		return fmt.Errorf("test file add: %s", err)
	}

	err = testRefsList(tdir, bin)
	if err != nil {
		return fmt.Errorf("test refs list: %s", err)
	}
	stump.Log("success! tests all passed.")

	return nil
}

func versionMatch(a, b string) bool {
	if strings.HasSuffix(b, "-dev") && strings.Contains(a, "-pre") {
		af := strings.Split(a, "-")[0]
		bf := strings.Split(b, "-")[0]
		return af == bf
	}

	return a == b
}

func testFileAdd(tdir, bin string) error {
	stump.VLog("  - checking that we can add and cat a file")
	text := []byte("hello world! This node should work")
	testFile := filepath.Join(tdir, "/test.txt")
	err := ioutil.WriteFile(testFile, text, 0644)
	if err != nil {
		stump.Error("testfileadd could not create test file: %s", err)
	}

	c := exec.Command(bin, "add", "-q", "--progress=false", testFile)
	if runtime.GOOS == "windows" {
		c.Env = os.Environ()
	}
	c.Env = util.ReplaceEnvVarIfExists(c.Env, "IPFS_PATH", tdir)
	out, err := c.CombinedOutput()
	if err != nil {
		stump.Error("testfileadd fail: %s", err)
		stump.Error(string(out))
		return err
	}

	hash := strings.Trim(string(out), "\n \t\r")
	fiout, err := runCmd(tdir, bin, "cat", hash)
	if err != nil {
		return err
	}

	if fiout != string(text) {
		return fmt.Errorf("add/cat check failed")
	}

	return nil
}

func testRefsList(tdir, bin string) error {
	stump.VLog("  - checking that file shows up in ipfs refs local")
	c := exec.Command(bin, "refs", "local")
	if runtime.GOOS == "windows" {
		c.Env = os.Environ()
	}
	c.Env = util.ReplaceEnvVarIfExists(c.Env, "IPFS_PATH", tdir)
	out, err := c.CombinedOutput()
	if err != nil {
		stump.Error("testfileadd fail: %s", err)
		stump.Error(string(out))
		return err
	}

	hashes := strings.Split(string(out), "\n")
	exp := "QmTFJQ68kaArzsqz2Yjg1yMyEA5TXTfNw6d9wSFhxtBxz2"
	var found bool
	for _, h := range hashes {
		if h == exp {
			found = true
			break
		}
	}
	if !found {
		return fmt.Errorf("expected to see %s in the local refs!", exp)
	}

	return nil
}
