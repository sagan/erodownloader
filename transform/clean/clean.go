package clean

import (
	"io/fs"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util"
)

var tmpfiles = []string{
	"desktop.ini",
	"Thumbs.db",
	".DS_Store",
}

// 删除临时文件。例如: Desktop.ini 等。
func Transformer(tc *transform.TransformerContext) (changed bool, err error) {
	err = filepath.WalkDir(tc.Dir, func(path string, d fs.DirEntry, err error) error {
		if err != nil {
			return err
		}
		if path == tc.Dir {
			return nil
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			if !slices.Contains(tmpfiles, d.Name()) {
				return nil
			}
		}
		if d.IsDir() {
			return nil
		}
		basename := filepath.Base(path)
		ext := filepath.Ext(path)
		if slices.Contains(tmpfiles, basename) {
			changed = true
			tc.Log("Remove %s", path)
			return os.Remove(path)
		}
		if ext == ".aria2" && !util.FileExists(path[:len(path)-len(ext)]) {
			changed = true
			tc.Log("Remove alone %s", path)
			return os.Remove(path)
		}
		return nil
	})
	return
}

func init() {
	transform.Register(&transform.Transformer{
		Name:   "clean",
		Action: Transformer,
	})
}
