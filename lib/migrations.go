package lib

import (
	"fmt"
	"io/ioutil"
	"os"
	"os/exec"
	"path/filepath"
	"strconv"
	"strings"

	util "github.com/ipfs/ipfs-update/util"
	stump "github.com/whyrusleeping/stump"
)

const migrations = "fs-repo-migrations"

func CheckMigration() error {
	stump.Log("checking if repo migration is needed...")
	p := util.IpfsDir()

	vfilePath := filepath.Join(p, "version")
	_, err := os.Stat(vfilePath)
	if os.IsNotExist(err) {
		stump.VLog("  - no prexisting repo to migrate")
		return nil
	}

	oldverB, err := ioutil.ReadFile(vfilePath)
	if err != nil {
		return err
	}

	oldver := strings.Trim(string(oldverB), "\n \t")
	stump.VLog("  - old repo version is", oldver)

	nbinver, err := util.RunCmd("", "ipfs", "version", "--repo")
	if err != nil {
		stump.Log("Failed to check new binary repo version.")
		stump.VLog("Reason: ", err)
		stump.Log("This is not an error.")
		stump.Log("This just means that you may have to manually run the migration")
		stump.Log("You will be prompted to do so upon starting the ipfs daemon if necessary")
		return nil
	}

	stump.VLog("  - repo version of new binary is ", nbinver)

	if oldver != nbinver {
		stump.Log("  check complete, migration required.")
		return RunMigration(oldver, nbinver)
	}

	stump.VLog("  check complete, no migration required.")

	return nil
}

func RunMigration(oldv, newv string) error {
	migrateBin := util.OsExeFileName("fs-repo-migrations")
	stump.VLog("  - checking for migrations binary...")
	_, err := exec.LookPath(migrateBin)
	if err != nil {
		stump.VLog("  - migrations not found on system, attempting to install")
		loc, err := GetMigrations()
		if err != nil {
			return err
		}

		migrateBin = loc
	}

	// check to make sure migrations binary supports our target version
	migrateBin, err = verifyMigrationSupportsVersion(migrateBin, newv)
	if err != nil {
		return err
	}

	cmd := exec.Command(migrateBin, "-to", newv, "-y", "--revert-ok")

	cmd.Stdout = stump.LogOut
	cmd.Stderr = stump.ErrOut

	stump.Log("running migration: '%s -to %s -y'", migrateBin, newv)

	err = cmd.Run()
	if err != nil {
		return fmt.Errorf("migration failed: %s", err)
	}

	stump.Log("migration succeeded!")
	return nil
}

func GetMigrations() (string, error) {
	latest, err := GetLatestVersion(util.IpfsVersionPath, migrations)
	if err != nil {
		return "", fmt.Errorf("getting latest version of fs-repo-migrations: %s", err)
	}

	dir, err := ioutil.TempDir("", "ipfs-update-migrate")
	if err != nil {
		return "", fmt.Errorf("tempdir: %s", err)
	}

	out := filepath.Join(dir, util.OsExeFileName(migrations))

	err = GetBinaryForVersion(migrations, migrations, util.IpfsVersionPath, latest, out)
	if err != nil {
		stump.Error("getting migrations binary: %s", err)

		stump.Log("could not find or install fs-repo-migrations, please manually install it")
		stump.Log("before running ipfs-update again.")
		return "", fmt.Errorf("failed to find migrations binary")
	}

	err = os.Chmod(out, 0755)
	if err != nil {
		return "", err
	}

	return out, nil
}

func getMigrationsGoGet() (string, error) {
	stump.VLog("  - fetching migrations using 'go get'")
	cmd := exec.Command("go", "get", "-u", "github.com/ipfs/fs-repo-migrations")
	out, err := cmd.CombinedOutput()
	if err != nil {
		return "", fmt.Errorf("%s %s", string(out), err)
	}
	stump.VLog("  - success. verifying...")

	// verify we can see the binary now
	p, err := exec.LookPath(util.OsExeFileName("fs-repo-migrations"))
	if err != nil {
		return "", fmt.Errorf("install succeeded, but failed to find binary afterwards. (%s)", err)
	}
	stump.VLog("  - fs-repo-migrations now installed at %s", p)

	return filepath.Join(os.Getenv("GOPATH"), "bin", migrations), nil
}

func verifyMigrationSupportsVersion(fsrbin, v string) (string, error) {
	stump.VLog("  - verifying migration supports version %s", v)
	vn, err := strconv.Atoi(v)
	if err != nil {
		return fsrbin, fmt.Errorf("given migration version was not a number: %q", v)
	}

	sn, err := migrationsVersion(fsrbin)
	if err != nil {
		return fsrbin, err
	}

	if sn >= vn {
		return fsrbin, nil
	}

	stump.VLog("  - migrations doesnt support version %s, attempting to update", v)
	fsrbin, err = GetMigrations()
	if err != nil {
		return fsrbin, err
	}

	stump.VLog("  - migrations updated")

	sn, err = migrationsVersion(fsrbin)
	if err != nil {
		return fsrbin, err
	}

	if sn >= vn {
		return fsrbin, nil
	}

	return fsrbin, fmt.Errorf("no known migration supports version %s", v)
}

func migrationsVersion(bin string) (int, error) {
	out, err := exec.Command(bin, "-v").CombinedOutput()
	if err != nil {
		return 0, fmt.Errorf("failed to check migrations version: %s", err)
	}

	vs := strings.Trim(string(out), " \n\t")
	vn, err := strconv.Atoi(vs)
	if err != nil {
		return 0, fmt.Errorf("migrations binary version check did not return a number")
	}

	return vn, nil
}
