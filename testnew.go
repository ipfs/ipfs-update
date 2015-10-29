package main

import (
	"bytes"
	"encoding/json"
	"fmt"
	"io"
	"io/ioutil"
	"net"
	"os"
	"os/exec"
	"path"
	"strings"
	"time"

	. "QmQCG86gb1evuzBTkjKuZ22KVkT3yf7vhDqvZXrMjVUucT/stump"
)

func runCmd(p, bin string, args ...string) (string, error) {
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

type daemon struct {
	p      *os.Process
	stderr io.WriteCloser
	stdout io.WriteCloser
}

func (d *daemon) Close() error {
	err := d.p.Kill()
	if err != nil {
		Error("error killing daemon: %s", err)
		return err
	}

	_, err = d.p.Wait()
	if err != nil {
		Error("error waiting on killed daemon: %s", err)
		return err
	}

	d.stderr.Close()
	d.stdout.Close()
	return nil
}

func tweakConfig(ipfspath string) error {
	cfgpath := path.Join(ipfspath, "config")
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
	cmd := exec.Command(bin, "daemon")

	stdout, err := os.Create(path.Join(p, "daemon.stdout"))
	if err != nil {
		return nil, err
	}

	stderr, err := os.Create(path.Join(p, "daemon.stderr"))
	if err != nil {
		return nil, err
	}

	cmd.Env = []string{"IPFS_PATH=" + p}
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
	apifile := path.Join(ipfspath, "api")
	var endpoint string
	for i := 0; i < 10; i++ {
		val, err := ioutil.ReadFile(apifile)
		if os.IsNotExist(err) {
			if i == 9 {
				return fmt.Errorf("failed to find api file")
			}
			time.Sleep(time.Millisecond * (100 * time.Duration(i+1)))

			continue
		} else if err != nil {
			return err
		}

		endpoint = string(val)
		break
	}

	parts := strings.Split(endpoint, "/")
	port := parts[len(parts)-1]

	for i := 0; i < 10; i++ {
		_, err := net.Dial("tcp", "localhost:"+port)
		if err == nil {
			return nil
		}

		time.Sleep(time.Millisecond * (100 * time.Duration(i+1)))
	}

	return fmt.Errorf("failed to come online")
}

func TestBinary(bin, version string) error {
	// make sure binary is executable
	err := os.Chmod(bin, 0755)
	if err != nil {
		return err
	}

	staging := path.Join(ipfsDir(), "update-staging")
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
			Error("error cleaning up staging directory: ", err)
		}
	}(tdir)

	_, err = runCmd(tdir, bin, "init")
	if err != nil {
		return fmt.Errorf("error initializing with new binary: %s", err)
	}

	rversion, err := runCmd(tdir, bin, "version")
	if err != nil {
		return err
	}

	if rversion != "ipfs version "+version[1:] {
		return fmt.Errorf("version didnt match")
	}

	// set up ports in config so we dont interfere with an already running daemon
	err = tweakConfig(tdir)
	if err != nil {
		return err
	}

	daemon, err := StartDaemon(tdir, bin)
	if err != nil {
		return fmt.Errorf("error starting daemon: %s", err)
	}
	defer daemon.Close()

	// test things against the daemon
	err = testFileAdd(tdir, bin)
	if err != nil {
		return err
	}

	return nil
}

func testFileAdd(tdir, bin string) error {
	text := "hello world! This node should work"
	data := bytes.NewBufferString(text)
	c := exec.Command(bin, "add", "-q")
	c.Env = []string{"IPFS_PATH=" + tdir}
	c.Stdin = data
	out, err := c.CombinedOutput()
	if err != nil {
		Error("testfileadd fail: %s", err)
		Error(string(out))
		return err
	}

	hash := strings.Trim(string(out), "\n \t")
	fiout, err := runCmd(tdir, bin, "cat", hash)
	if err != nil {
		return err
	}

	if fiout != text {
		return fmt.Errorf("add/cat check failed")
	}

	return nil
}
