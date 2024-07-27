package denesting

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util"
)

// 去除多层套娃文件夹结构。
func Transformer(tc *transform.TransformerContext) (changed bool, err error) {
	tmpdir := filepath.Join(tc.Dir, transform.TMP_DIR)
	if err = os.RemoveAll(tmpdir); err != nil {
		return
	}
	contentFiles, err := os.ReadDir(tc.Dir)
	if err != nil || len(contentFiles) != 1 ||
		!contentFiles[0].IsDir() || strings.HasPrefix(contentFiles[0].Name(), ".") {
		return
	}
	if err = util.MakeCleanTmpDir(tmpdir); err != nil {
		err = fmt.Errorf("failed to make tmpdir: %w", err)
		return
	}
	defer os.RemoveAll(tmpdir)

	tc.Log("denesting dir %s", contentFiles[0].Name())
	changed = true
	originalContentDir := filepath.Join(tc.Dir, ".orig."+contentFiles[0].Name())
	if err = atomic.ReplaceFile(filepath.Join(tc.Dir, contentFiles[0].Name()), originalContentDir); err != nil {
		return
	}
	contentDir := originalContentDir
	for {
		contentFiles, err = os.ReadDir(contentDir)
		if len(contentFiles) == 1 && contentFiles[0].IsDir() {
			contentDir = filepath.Join(contentDir, contentFiles[0].Name())
			continue
		}
		break
	}
	if contentFiles, err = os.ReadDir(contentDir); err != nil {
		return
	}
	for _, file := range contentFiles {
		if err = atomic.ReplaceFile(filepath.Join(contentDir, file.Name()),
			filepath.Join(tc.Dir, file.Name())); err != nil {
			return
		}
	}
	err = os.RemoveAll(originalContentDir)
	return
}

func init() {
	transform.Register(&transform.Transformer{
		Name:   "denesting",
		Action: Transformer,
	})
}
