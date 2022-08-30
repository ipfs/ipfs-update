package util

import (
	"os"
	"path"
	"testing"
)

func TestApiEndpoint(t *testing.T) {
	fakeHome, err := os.MkdirTemp("", "testhome")
	if err != nil {
		panic(err)
	}
	defer os.RemoveAll(fakeHome)
	defer os.Unsetenv("HOME")
	defer os.Unsetenv("IPFS_PATH")

	os.Setenv("IPFS_PATH", "")
	os.Setenv("HOME", fakeHome)
	fakeIpfs := path.Join(fakeHome, ".ipfs")

	err = os.Mkdir(fakeIpfs, os.ModePerm)
	if err != nil {
		panic(err)
	}

	_, err = ApiEndpoint("")
	if err == nil {
		t.Fatal("expected error when missing api file")
	}

	apiPath := path.Join(fakeIpfs, apiFile)
	err = os.WriteFile(apiPath, []byte("bad-data"), 0o644)
	if err != nil {
		panic(err)
	}

	_, err = ApiEndpoint("")
	if err == nil {
		t.Fatal("expected error when bad data")
	}

	err = os.WriteFile(apiPath, []byte("/ip4/127.0.0.1/tcp/5001"), 0o644)
	if err != nil {
		panic(err)
	}

	val, err := ApiEndpoint("")
	if err != nil {
		t.Fatal(err)
	}
	if val != "127.0.0.1:5001" {
		t.Fatal("got unexpected value:", val)
	}

	val2, err := ApiEndpoint(fakeIpfs)
	if err != nil {
		t.Fatal(err)
	}
	if val2 != val {
		t.Fatal("expected", val, "got", val2)
	}
}
