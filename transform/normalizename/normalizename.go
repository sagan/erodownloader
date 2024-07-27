package normalizename

import (
	"os"
	"path/filepath"
	"strings"

	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util/helper"
)

// 规格化文件名
func Transformer(tc *transform.TransformerContext) (changed bool, err error) {
	entries, err := os.ReadDir(tc.Dir)
	if err != nil {
		return
	}
	var pathes []string
	for _, entry := range entries {
		if strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		pathes = append(pathes, filepath.Join(tc.Dir, entry.Name()))
	}
	renamed, err := helper.NormalizeName(false, pathes...)
	if renamed > 0 {
		changed = true
	}
	return
}

func init() {
	transform.Register(&transform.Transformer{
		Name:   "normalizename",
		Action: Transformer,
	})
}
