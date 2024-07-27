package noempty

import (
	"io/fs"
	"path/filepath"
	"strings"

	"github.com/sagan/erodownloader/transform"
	"golang.org/x/exp/slices"
)

const MIN_SIZE = 100 * 1024 // 100KiB

var IgnoreFiles = []string{"desktop.ini", "Thumbs.db"}
var IgnoreExts = []string{".nfo"}
var IgnoreNames = []string{"cover"}

// 确保文件夹非空
func Transformer(tc *transform.TransformerContext) (changed bool, err error) {
	totalSize := int64(0)
	filepath.Walk(tc.Dir, func(path string, info fs.FileInfo, err error) error {
		if path == tc.Dir {
			return err
		}
		basename := info.Name()
		if strings.HasPrefix(basename, ".") {
			if info.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if err != nil {
			return err
		}
		if info.IsDir() {
			return nil
		}
		ext := strings.ToLower(filepath.Ext(basename))
		base := strings.ToLower(basename[:len(basename)-len(ext)])
		if slices.Contains(IgnoreFiles, basename) ||
			slices.Contains(IgnoreExts, ext) || slices.Contains(IgnoreNames, base) {
			return nil
		}
		totalSize += info.Size()
		if totalSize >= MIN_SIZE {
			return fs.SkipAll
		}
		return nil
	})

	if totalSize < MIN_SIZE {
		err = transform.ErrInvalid
	}
	return
}

func init() {
	transform.Register(&transform.Transformer{
		Name:   "noempty",
		Action: Transformer,
	})
}
