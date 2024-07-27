package correctext

import (
	"fmt"
	"os"
	"path/filepath"
	"slices"
	"strings"

	"github.com/h2non/filetype"
	"github.com/natefinch/atomic"

	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/helper"
	"github.com/sagan/erodownloader/util/stringutil"
)

const HEADER_SIZE = 512 * 1024 // 512KiB

// 根据 magic number 检测文件类型，将部分文件扩展名纠正为正确的。
func Transformer(tc *transform.TransformerContext) (changed bool, err error) {
	entries, err := os.ReadDir(tc.Dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		basename := filepath.Base(entry.Name())
		ext := filepath.Ext(basename)
		base := basename[:len(basename)-len(ext)]

		exts := constants.ArchiveExts
		if stringutil.HasAnySuffix(entry.Name(), exts...) {
			var header []byte
			fullpath := filepath.Join(tc.Dir, entry.Name())
			header, err = helper.ReadFileHeader(fullpath, HEADER_SIZE)
			if err != nil {
				return
			}
			kind, _ := filetype.Match(header)
			if kind.Extension == "" {
				continue
			}
			newext := strings.ToLower(kind.Extension)
			if !strings.HasPrefix(newext, ".") {
				newext = "." + newext
			}
			if !slices.Contains(exts, newext) || strings.EqualFold(newext, ext) {
				continue
			}
			newpath := filepath.Join(tc.Dir, base+newext)
			if util.FileExists(newpath) {
				err = fmt.Errorf("failed to rename %q => %q: target already exists", fullpath, newpath)
				return
			}
			changed = true
			if err = atomic.ReplaceFile(fullpath, newpath); err != nil {
				err = fmt.Errorf("failed to rename %q => %q: %w", fullpath, newpath, err)
				return
			}
			tc.Log("renamed %q => %q", fullpath, newpath)
		}
	}
	return
}

func init() {
	transform.Register(&transform.Transformer{
		Name:   "correctext",
		Action: Transformer,
	})
}
