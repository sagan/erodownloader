package decensorship

import (
	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/transform/executor"
)

const SIZE_LIMIT = 100 * 1024 * 1024 // 最大处理的文件大小限制 (100MiB)

var textualExts = []string{".txt"}

func init() {
	executorOptions := &executor.ExecutorOptions{
		Ext:          textualExts,
		MaxSize:      SIZE_LIMIT,
		ContentsFunc: textNormalize,
	}
	transform.Register(&transform.Transformer{
		Name:   "text",
		Action: executorOptions.Transformer,
	})
}
