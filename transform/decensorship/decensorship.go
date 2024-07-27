package decensorship

import (
	"fmt"
	"io/fs"
	"path/filepath"
	"runtime"
	"strings"

	"github.com/natefinch/atomic"

	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util"
)

var extMapper = map[string]string{
	".rar2": ".rar",
	".zip2": ".rar",
	".mp42": ".mp4",
}

// 还原正确文件名。为了规避审查，一些网盘分享的资源文件名被刻意修改，
// 例如将后缀由 .rar 改为 .r_a_r 之类以防止在线解压。或将 .mp4 改为 .mp42 以防止在线播放。
// 同时会将文件名的扩展名改为全小写。
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
			return nil
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		newext := strings.ToLower(ext)
		newext = strings.ReplaceAll(newext, "_", "")
		newext = strings.ReplaceAll(newext, "-", "")
		if extMapper[newext] != "" {
			newext = extMapper[newext]
		}
		if ext == newext {
			return nil
		}
		newpath := path[:len(path)-len(ext)] + newext
		tc.Log("rename %s to %s", path, newpath)
		// Windows fs 读取文件名不区分大小写，但写入时区分。
		if (runtime.GOOS != "windows" || !strings.EqualFold(path, newpath)) && util.FileExists(newpath) {
			return fmt.Errorf("target file already exists")
		}
		changed = true
		return atomic.ReplaceFile(path, newpath)
	})
	return
}

func init() {
	transform.Register(&transform.Transformer{
		Name:   "decensorship",
		Action: Transformer,
	})
}
