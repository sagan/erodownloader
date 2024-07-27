package nocredit

import (
	"crypto/sha256"
	"encoding/hex"
	"io/fs"
	"os"
	"path/filepath"
	"strings"

	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util/stringutil"
)

const MAX_SIZE = 1 * 1024 * 1024 // 1MiB

type CreditFile struct {
	Prefix  []string // UTF-8 string prefixes. Matches if any prefix match.
	Sha256  string   // sha-256 hash of file contents, hex string (lower case)
	Size    int64
	MaxSize int64
}

var CreditFiles = map[string][]*CreditFile{
	"Read_Me.txt": {
		{
			Prefix: []string{
				`本资源为免费资源，如果你是从倒狗手上获取的请立即举报+拉黑` + "\n",
				`如果喜欢且有财力的，请支持正版，` + "\n",
				`出自asmrconnecting,联系邮箱:admin@asmrconnecting.xyz` + "\n",
				`联系邮箱:admin@asmrconnecting.xyz` + "\n",
			},
			MaxSize: 2 * 1024, // 2KiB
		},
	},
}

// 移除一些常见的发布者 credit 文件。
func Transformer(tc *transform.TransformerContext) (changed bool, err error) {
	entries, err := os.ReadDir(tc.Dir)
	if err != nil {
		return
	}
	for _, entry := range entries {
		if entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
			continue
		}
		creditFiles := CreditFiles[entry.Name()]
		if creditFiles == nil {
			continue
		}
		filepath := filepath.Join(tc.Dir, entry.Name())
		var stat fs.FileInfo
		stat, err = os.Stat(filepath)
		if err != nil {
			return
		}
		if stat.Size() > MAX_SIZE {
			continue
		}
		var contents []byte
		contents, err = os.ReadFile(filepath)
		if err != nil {
			return
		}
		sha256er := sha256.New()
		if _, err = sha256er.Write(contents); err != nil {
			return
		}
		sha256Hash := hex.EncodeToString(sha256er.Sum(nil))
		contentsStr := stringutil.StringFromBytes(contents)
		match := false
		for _, file := range creditFiles {
			if file.Size > 0 && stat.Size() != file.Size ||
				file.MaxSize > 0 && stat.Size() > file.MaxSize {
				continue
			}
			if file.Sha256 != "" {
				if sha256Hash == file.Sha256 {
					match = true
					break
				} else {
					continue
				}
			}
			for _, prefix := range file.Prefix {
				if prefix != "" && strings.HasPrefix(contentsStr, prefix) {
					match = true
					break
				}
			}
		}
		if !match {
			continue
		}
		tc.Log("Remove credit file %s", entry.Name())
		changed = true
		if err = os.Remove(filepath); err != nil {
			return
		}
	}
	return
}

func init() {
	transform.Register(&transform.Transformer{
		Name:   "nocredit",
		Action: Transformer,
	})
}
