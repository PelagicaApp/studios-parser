package paths

import (
	"path/filepath"
	"runtime"
)

var rootDir = func() string {
	_, currentFile, _, _ := runtime.Caller(0)
	return filepath.Join(filepath.Dir(currentFile), "..", "..")
}()

var DataDir = filepath.Join(rootDir, "data")
var TempDir = filepath.Join(rootDir, "temp")
