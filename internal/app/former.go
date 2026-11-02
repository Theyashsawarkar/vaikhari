package app

import (
	"os"
	"path/filepath"
)

// MoveFormer: vaikhari was called prasar up to v0.1, so its folders
// (accounts, drafts, scheduled posts) and its database move to the new name
// the first time the command runs. Only main calls it, never a test.
func MoveFormer() {
	if os.Getenv("VAIKHARI_HOME") != "" {
		return
	}
	p := DefaultPaths()
	for _, dir := range []string{p.Config, p.Data, p.State} {
		moveFormer(dir, "prasar")
	}
	// the database inside kept the old name too
	for _, ext := range []string{"", "-wal", "-shm"} {
		from, to := filepath.Join(p.Data, "prasar.db"+ext), filepath.Join(p.Data, "vaikhari.db"+ext)
		if _, err := os.Stat(to); err != nil {
			_ = os.Rename(from, to)
		}
	}
}

func moveFormer(dir, old string) {
	from := filepath.Join(filepath.Dir(dir), old)
	if _, err := os.Stat(dir); err == nil {
		return
	}
	if fi, err := os.Lstat(from); err != nil || !fi.IsDir() {
		return
	}
	_ = os.Rename(from, dir)
}
