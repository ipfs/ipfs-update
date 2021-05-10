package util

import (
	"os"
	"path"
	"time"

	"golang.org/x/sys/windows"
)

func init() {
	// attempts to remove path or move it to the systems temporary directory
	forceRemove = winForceRemove
	InsideGUI = winInsideGUI
}

func winForceRemove(filePath string) error {
	_, err := os.Stat(filePath)
	if err != nil {
		return nil
	}
	if err = os.Remove(filePath); err != nil {
		// fallback for when file is still in use (likely the daemon is up)
		finalpath := path.Join(os.TempDir(), time.Now().Format("2006.01.02-15.04.05")+" "+path.Base(filePath))
		return windows.MoveFileEx(windows.StringToUTF16Ptr(filePath), windows.StringToUTF16Ptr(finalpath), windows.MOVEFILE_COPY_ALLOWED)
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
