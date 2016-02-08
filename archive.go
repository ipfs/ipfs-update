package main

import (
	"archive/tar"
	"archive/zip"
	"compress/gzip"
	"fmt"
	"io"
	"os"

	stump "github.com/whyrusleeping/stump"
)

func unpackArchive(path, out, atype string) error {
	switch atype {
	case "zip":
		return unpackZip(path, out)
	case "tar.gz":
		return unpackTgz(path, out)
	default:
		return fmt.Errorf("unrecognized archive type: %s", atype)
	}
}

func unpackTgz(path, out string) error {
	fi, err := os.Open(path)
	if err != nil {
		return err
	}
	defer fi.Close()

	gzr, err := gzip.NewReader(fi)
	if err != nil {
		return err
	}

	defer gzr.Close()

	var bin io.Reader
	tarr := tar.NewReader(gzr)

loop:
	for {
		th, err := tarr.Next()
		switch err {
		default:
			return err
		case io.EOF:
			break loop
		case nil:
			// continue
		}

		if th.Name == "go-ipfs/ipfs" {
			bin = tarr
			break
		}
	}

	if bin == nil {
		return fmt.Errorf("no ipfs binary found in downloaded archive")
	}

	return writeToPath(bin, out)
}

func writeToPath(rc io.Reader, out string) error {
	stump.VLog("  - extracting binary to tempdir: ", out)
	binfi, err := os.Create(out)
	if err != nil {
		return fmt.Errorf("error opening tmp bin path '%s': %s", out, err)
	}
	defer binfi.Close()

	_, err = io.Copy(binfi, rc)
	if err != nil {
		return err
	}

	return nil
}

func unpackZip(path, out string) error {
	stump.VLog("  - unpacking zip to %q", out)
	zipr, err := zip.OpenReader(path)
	if err != nil {
		return fmt.Errorf("error opening zipreader: %s", err)
	}

	defer zipr.Close()

	var bin io.ReadCloser
	for _, fis := range zipr.File {
		if fis.Name == "go-ipfs/ipfs.exe" {
			rc, err := fis.Open()
			if err != nil {
				return fmt.Errorf("error extracting binary from archive: %s", err)
			}

			bin = rc
		}
	}

	return writeToPath(bin, out)
}
