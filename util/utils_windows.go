package util

import (
	"os"
	"path/filepath"
	"time"

	"golang.org/x/sys/windows"
)

func init() {
	// attempts to remove path or move it to the systems temporary directory
	forceRemove = winForceRemove
	InsideGUI = winInsideGUI
}

func winForceRemove(path string) error {
	if _, err := os.Stat(path); err == nil {
		if err = os.Remove(path); err != nil {
			// fallback for when file is still in use (likely the daemon is up)
			finalpath := filepath.Join(os.TempDir(), time.Now().Format("2006.01.02-15.04.05")+" "+filepath.Base(path))
			if err = windows.MoveFileEx(windows.StringToUTF16Ptr(path), windows.StringToUTF16Ptr(finalpath), windows.MOVEFILE_COPY_ALLOWED); err != nil {
				return err
			}
		}
	}
	return nil
}
func winInsideGUI() bool {
	conhostInfo := &windows.ConsoleScreenBufferInfo{}
	if err := windows.GetConsoleScreenBufferInfo(windows.Stdout, conhostInfo); err != nil {
		return false
	}

	if (conhostInfo.CursorPosition.X | conhostInfo.CursorPosition.Y) == 0 {
		// console cursor has not moved prior to our execution
		// high probability that we're not in a terminal
		return true
	}

	return false
}
