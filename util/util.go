package util

import (
	"bytes"
	"crypto/md5"
	"encoding/hex"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"io/fs"
	"math/rand"
	"mime"
	"os"
	"os/exec"
	"path"
	"path/filepath"
	"reflect"
	"regexp"
	"runtime"
	"slices"
	"strconv"
	"strings"
	"time"

	"golang.org/x/exp/constraints"
	"golang.org/x/net/html"

	"github.com/PuerkitoBio/goquery"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/util/stringutil"
)

func FormatTime(t int64) string {
	return time.Unix(t, 0).Format("2006-01-02 15:04:05")
}

// Return t unconditionally.
func First[T any](t T, args ...any) T {
	return t
}

func FirstNonZeroArg[T comparable](args ...T) T {
	var empty T
	for _, t := range args {
		if t != empty {
			return t
		}
	}
	return empty
}

// Panic if err is not nil, otherwise return t
func Unwrap[T any](t T, err error) T {
	if err != nil {
		panic(err)
	}
	return t
}

// Unmarshal source as json of type T
func UnmarshalJson[T any](source []byte) (T, error) {
	var target T
	if err := json.Unmarshal(source, &target); err != nil {
		return target, err
	}
	return target, nil
}

func ParseInt[T constraints.Integer](s string, defaultValue T) T {
	if s != "" {
		if i, err := strconv.Atoi(s); err == nil {
			return T(i)
		}
	}
	return defaultValue
}

func OmitemptySlice[T comparable](slice []T) (list []T) {
	var emptyvalue T
	for _, entry := range slice {
		if entry != emptyvalue {
			list = append(list, entry)
		}
	}
	return list
}

func UniqueSlice[T comparable](slice []T) (list []T) {
	keys := map[T]struct{}{}
	for _, entry := range slice {
		if _, ok := keys[entry]; !ok {
			keys[entry] = struct{}{}
			list = append(list, entry)
		}
	}
	return list
}

func Map[T1 any, T2 any](ss []T1, mapper func(T1) T2) (ret []T2) {
	for _, s := range ss {
		ret = append(ret, mapper(s))
	}
	return
}

// Clean & normaliza & sanitize p path.
// 1. path.Clean.
// 2. Replace \ with / .
// 3. Clean each part of the path, replace invalid chars with alternatives, truncate too long names.
func CleanPath(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	p = path.Clean(p)
	segments := strings.Split(p, "/")
	for i := range segments {
		segments[i] = CleanBasename(segments[i])
	}
	return strings.Join(segments, "/")
}

// Similar to CleanPath but treat p as a file path (the basename of p contains ext),
// and try to preserve ext in basename.
func CleanFilePath(p string) string {
	p = strings.ReplaceAll(p, `\`, `/`)
	p = path.Clean(p)
	segments := strings.Split(p, "/")
	lastSegment := segments[len(segments)-1]
	segments = segments[:len(segments)-1]
	for i := range segments {
		segments[i] = CleanBasename(segments[i])
	}
	lastSegment = CleanFileBasename(lastSegment)
	segments = append(segments, lastSegment)
	return strings.Join(segments, "/")
}

// Return a cleaned safe base filename component.
// 1. Replace invalid chars with alternatives (e.g. "?" => "？").
// 2. CleanTitle (clean \r, \n and other invisiable chars then TrimSpace).
func CleanBasenameComponent(name string) string {
	name = constants.FilenameRestrictedCharacterReplacer.Replace(name)
	name = stringutil.CleanTitle(name)
	return name
}

// Return a cleaned safe base filename (without path).
// 1. CleanBaseFilenameComponent.
// 2. Clean trailing dot (".") (Windows does NOT allow dot in the end of filename)
// 3. TrimSpace
// 4. Truncate name to at most 240 (UTF-8 string) bytes.
func CleanBasename(name string) string {
	name = CleanBasenameComponent(name)
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	name = strings.TrimSpace(name)
	return stringutil.StringPrefixInBytes(name, constants.FILENAME_MAX_LENGTH)
}

// Similar to CleanBaseName, but treats name as a filename (base+ext) and tries to preserve ext.
// It also removes spaces between base and ext.
func CleanFileBasename(name string) string {
	name = CleanBasenameComponent(name)
	for len(name) > 0 && name[len(name)-1] == '.' {
		name = name[:len(name)-1]
	}
	name = strings.TrimSpace(name)
	ext := path.Ext(name)
	if len(ext) > 14 || strings.ContainsAny(ext, " ") {
		return stringutil.StringPrefixInBytes(name, constants.FILENAME_MAX_LENGTH)
	}
	base := name[:len(name)-len(ext)]
	base = strings.TrimSpace(base)
	return stringutil.StringPrefixInBytes(base, constants.FILENAME_MAX_LENGTH-len(ext)) + ext
}

func Sleep(seconds int) {
	timeout := time.Second*time.Duration(seconds) + time.Millisecond*time.Duration(rand.Intn(1000))
	time.Sleep(timeout)
}

// Check whether a file (or dir) with name exists in file system
func FileExists(name string) bool {
	if _, err := os.Stat(name); err == nil || !errors.Is(err, fs.ErrNotExist) {
		return true
	}
	return false
}

// Make a new empty temp dir at tmpdir location.
// If tmpdir already exists, clean it first(remove itself with all contents inside it).
func MakeCleanTmpDir(tmpdir string) error {
	if FileExists(tmpdir) {
		if err := os.RemoveAll(tmpdir); err != nil {
			return err
		}
	}
	return os.MkdirAll(tmpdir, 0700)
}

// Return count of variable in vars that fulfil the condition that variable is non-zero value
func CountNonZeroVariables(vars ...any) (cnt int) {
	for _, variable := range vars {
		if reflect.ValueOf(variable).Kind() == reflect.Func {
			if reflect.ValueOf(variable).Pointer() != 0 {
				cnt++
			}
			continue
		}
		switch v := variable.(type) {
		case string:
			if v != "" {
				cnt++
			}
		case int:
			if v != 0 {
				cnt++
			}
		case int64:
			if v != 0 {
				cnt++
			}
		case float64:
			if v != 0 {
				cnt++
			}
		case bool:
			if v {
				cnt++
			}
		case []string:
			if len(v) > 0 {
				cnt++
			}
		default:
			panic("unsupported type")
		}
	}
	return
}

func GetFunctionName(i any) string {
	return runtime.FuncForPC(reflect.ValueOf(i).Pointer()).Name()
}

// Return filtered ss. The ret is nil if and only if ss is nil.
func FilterSlice[T any](ss []T, test func(T) bool) (ret []T) {
	if ss != nil {
		ret = []T{}
	}
	for _, s := range ss {
		if test(s) {
			ret = append(ret, s)
		}
	}
	return
}

// Check whether a file (or dir) with name + <any suffix in list> exists in file system.
// If exists, return name + suffix. Else, return empty string.
// If len(suffixes) == 0, return name if itself exists.
func ExistsFileWithAnySuffix(name string, suffixes ...string) string {
	if len(suffixes) == 0 {
		if FileExists(name) {
			return name
		}
		return ""
	}
	for _, suffix := range suffixes {
		if path := name + suffix; FileExists(path) {
			return path
		}
	}
	return ""
}

var commaSeperatorRegexp = regexp.MustCompile(`\s*,\s*`)

// split a csv like line to values. "a, b, c" => [a,b,c]
func SplitCsv(str string) []string {
	if str == "" {
		return nil
	}
	return commaSeperatorRegexp.Split(str, -1)
}

func PrintJson(output io.Writer, value any) error {
	bytes, err := json.MarshalIndent(value, "", "  ")
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	fmt.Fprintln(output, string(bytes))
	return nil
}

func Md5(input string) string {
	hash := md5.New()
	io.WriteString(hash, input)
	return hex.EncodeToString(hash.Sum(nil))
}

// typ: "Content-Type" header.
func GetExtFromType(typ string) string {
	if typ != "" {
		if exts, err := mime.ExtensionsByType(typ); err == nil && len(exts) > 0 {
			// Go 标准库返回的 exts 不是按常用顺序排列的。
			// 例如 image/jpeg : [.jfif .jpe .jpeg .jpg]
			for _, ext := range constants.CommonExts {
				if slices.Contains(exts, ext) {
					return ext
				}
			}
			return exts[0]
		}
	}
	return ""
}

// https://stackoverflow.com/questions/23350173
// copy non-empty field values from src to dst. dst and src must be pointors of same type of plain struct
func Assign(dst any, src any, excludeFieldIndexes []int) {
	dstValue := reflect.ValueOf(dst).Elem()
	srcValue := reflect.ValueOf(src).Elem()

	for i := 0; i < dstValue.NumField(); i++ {
		dstField := dstValue.Field(i)
		srcField := srcValue.Field(i)
		fieldType := dstField.Type()
		srcValue := reflect.Value(srcField)
		if slices.Contains(excludeFieldIndexes, i) {
			continue
		}
		if fieldType.Kind() == reflect.String && srcValue.String() == "" {
			continue
		}
		if fieldType.Kind() == reflect.Int64 && srcValue.Int() == 0 {
			continue
		}
		if fieldType.Kind() == reflect.Float64 && srcValue.Float() == 0 {
			continue
		}
		if fieldType.Kind() == reflect.Bool && !srcValue.Bool() {
			continue
		}
		if (fieldType.Kind() == reflect.Slice || fieldType.Kind() == reflect.Map ||
			fieldType.Kind() == reflect.Pointer) && srcValue.Pointer() == 0 {
			continue
		}
		dstField.Set(srcValue)
	}
}

// Similar to exec.LookPath, but also look up for dir of self executable file.
func LookPathWithSelfDir(name string) (string, error) {
	path, err := exec.LookPath(name)
	if err != nil {
		if self, err := os.Executable(); err == nil {
			selfDir := filepath.Dir(self)
			binarypath := filepath.Join(selfDir, name)
			var binaryExts []string
			// For simplicity, do not use PATHEXT.
			if runtime.GOOS == "windows" {
				binaryExts = []string{".com", ".exe", ".bat", ".cmd"}
			}
			if binarypath = ExistsFileWithAnySuffix(binarypath, binaryExts...); binarypath != "" {
				return binarypath, nil
			}
		}
	}
	return path, err
}

// Return deduplicated and combined text of dom elements.
// That is, if both parent and child node are in the same selection,
// it will only write once of parent's content to result, ignoring the (duplicate) child node.
// It will Clean result.
func DomSelectionText(s *goquery.Selection) string {
	var buf bytes.Buffer
	visited := map[*html.Node]struct{}{}
	s.Each(func(i int, s *goquery.Selection) {
		for p := s.Nodes[0]; p != nil; {
			if _, ok := visited[p]; ok {
				return
			}
			p = p.Parent
		}
		visited[s.Nodes[0]] = struct{}{}
		buf.WriteString(s.Text())
	})
	return stringutil.Clean(buf.String())
}
