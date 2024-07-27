package executor

import (
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"slices"
	"strings"

	"github.com/google/shlex"
	"github.com/natefinch/atomic"

	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/helper"
)

const INPUT_PLACEHOLDER = "{{input}}"
const OUTPUT_PLACEHOLDER = "{{output}}"
const BASE_PLACEHOLDER = "{{base}}"
const EXT_PLACEHOLDER = "{{ext}}"

type ExecutorOptions struct {
	// do create hardlink for each file before handling it. a workaround.
	// the hardlink file will be inside tmpdir and with name <hash>.ext.
	// <hash> is the md5 hash hex string of original basename without ext.
	Hardlink bool
	Binary   string // external binary
	Func     func(inputFile, outputFile string, options url.Values, logger transform.Logger) (changed bool, err error)
	// Original input file can be accessed from options.Get("input")
	ContentsFunc func(input []byte, options url.Values, logger transform.Logger) (output []byte, changed bool, err error)
	BinaryArgs   []string // must contains input & output placeholder
	Ext          []string // file exts that can be processed.
	MinSize      int64
	MaxSize      int64
	NewExt       string // replace with this new ext
	Output       string // output filename. placeholders: {{base}}, {{ext}}. E.g. "{{base}}.flac".
	// E.g. if have a value: []string{".vtt"},
	// if "foo.wav" was converted to "foo.flac" (a new name file),
	// then "foo.wav.vtt" file, if it exists, will be renamed to "foo.flac.vtt" accordingly.
	RenameAdditionalSuffixes []string
	// Only applies for Binary executor.
	// If returned newArgs is not nil and err is nil, will re-try execute binary use newArgs.
	// If return (nil,nil), will ignore current ierr and skip current input file.
	OnError func(combinedOutput []byte, ierr error, logger transform.Logger) (newArgs []string, err error)
	Test    func(path string) bool
}

var (
	ErrNoOutputFile = fmt.Errorf("no target output file exists")
)

// 对每个文件执行外部命令以替换文件内容
func Transformer(tc *transform.TransformerContext) (changed bool, err error) {
	binary := tc.Options.Get("binary")
	ext := tc.Options["ext"]
	binaryArgs, err := shlex.Split(tc.Options.Get("binary_args"))
	if binary == "" || err != nil || !slices.Contains(binaryArgs, INPUT_PLACEHOLDER) ||
		!slices.Contains(binaryArgs, OUTPUT_PLACEHOLDER) {
		return false, fmt.Errorf("invalid options")
	}
	options := &ExecutorOptions{
		Binary:     binary,
		BinaryArgs: binaryArgs,
		Ext:        ext,
	}
	return options.Transformer(tc)
}

// internal Transformer
func (options *ExecutorOptions) Transformer(tc *transform.TransformerContext) (changed bool, err error) {
	if util.CountNonZeroVariables(options.Binary, options.Func, options.ContentsFunc) != 1 {
		err = fmt.Errorf("invalid args: exact one of binary, func and contents_func must be set")
		return
	}
	binary := options.Binary
	if binary != "" {
		if customBinary := tc.Options.Get(binary + "_binary"); customBinary != "" {
			binary = customBinary
		}
		var binaryPath string
		if binaryPath, err = util.LookPathWithSelfDir(binary); err != nil {
			err = fmt.Errorf("binary %q not found", binary)
			return
		}
		binary = binaryPath
	}
	doBackup := tc.Options.Get("backup") == "1"
	tmpdir := filepath.Join(tc.Dir, transform.TMP_DIR)
	if err = util.MakeCleanTmpDir(tmpdir); err != nil {
		return
	}
	defer os.RemoveAll(tmpdir)
	err = filepath.WalkDir(tc.Dir, func(path string, d fs.DirEntry, err error) error {
		if path == tc.Dir {
			return err
		}
		if strings.HasPrefix(d.Name(), ".") {
			if d.IsDir() {
				return fs.SkipDir
			}
			return nil
		}
		if err != nil {
			return err
		}
		if d.IsDir() {
			return nil
		}
		ext := filepath.Ext(path)
		dirname := filepath.Dir(path)
		basename := filepath.Base(path)
		base := basename[:len(basename)-len(ext)]
		if len(options.Ext) > 0 && !slices.Contains(options.Ext, ext) {
			return nil
		}
		if options.MinSize > 0 || options.MaxSize > 0 {
			if info, err := d.Info(); err != nil {
				return err
			} else if info.Size() > options.MaxSize {
				tc.Log("skip file %q which is too large", path)
				return nil
			} else if info.Size() < options.MinSize {
				tc.Log("skip file %q which is too small", path)
				return nil
			}
		}
		if options.Test != nil && !options.Test(path) {
			return nil
		}
		targetFilename := basename
		if options.Output != "" {
			targetFilename = options.Output
			targetFilename = strings.ReplaceAll(targetFilename, BASE_PLACEHOLDER, base)
			targetFilename = strings.ReplaceAll(targetFilename, EXT_PLACEHOLDER, ext)
		} else if options.NewExt != "" {
			targetFilename = base + options.NewExt
		}
		targetFilePath := filepath.Join(dirname, targetFilename)
		tempFilePath := filepath.Join(tmpdir, targetFilename)
		if targetFilename != basename && util.FileExists(targetFilePath) {
			tc.Log("Skip %q due to target file %q already exists", path, targetFilePath)
			return nil
		}
		if util.FileExists(tempFilePath) {
			if err = os.Remove(tempFilePath); err != nil {
				return err
			}
		}
		if options.Func != nil {
			tc.Log("Execute func %s on %q", util.GetFunctionName(options.Func), path)
			if changed, err := options.Func(path, tempFilePath, tc.Options, tc.Log); err != nil {
				return err
			} else if !changed {
				return nil
			}
		} else if options.ContentsFunc != nil {
			tc.Log("Execute contents func %s on %q", util.GetFunctionName(options.ContentsFunc), path)
			tc.Options.Set("input", path)
			if contents, err := os.ReadFile(path); err != nil {
				return err
			} else if newContents, contentsChanged, err := options.ContentsFunc(contents, tc.Options, tc.Log); err != nil {
				return err
			} else if !contentsChanged {
				return nil
			} else if err = os.WriteFile(tempFilePath, newContents, 0600); err != nil {
				return err
			}
		} else {
			inputFile := path
			if options.Hardlink {
				hardlinkFile := helper.GetNewFilePath(tmpdir, util.Md5(base)+ext)
				if err = os.Link(inputFile, hardlinkFile); err != nil {
					return err
				}
				inputFile = hardlinkFile
			}
			binaryArgs := options.BinaryArgs
			var output []byte
			i := 0
			for {
				i++
				if i > 3 {
					err = fmt.Errorf("too many fails")
					break
				}
				if util.FileExists(tempFilePath) {
					if err = os.Remove(tempFilePath); err != nil {
						break
					}
				}
				var args []string
				for _, arg := range binaryArgs {
					switch arg {
					case INPUT_PLACEHOLDER:
						args = append(args, inputFile)
					case OUTPUT_PLACEHOLDER:
						args = append(args, tempFilePath)
					default:
						args = append(args, arg)
					}
				}
				tc.Log("Execute binary %q %v", binary, args)
				output, err = exec.Command(binary, args...).CombinedOutput()
				if err == nil && !util.FileExists(tempFilePath) {
					err = ErrNoOutputFile
				}
				if err == nil {
					tc.Log("Success executed")
					break
				} else if options.OnError == nil {
					tc.Log("Binary process exitted with error: %v", err)
					break
				} else if binaryArgs, err = options.OnError(output, err, tc.Log); err != nil {
					tc.Log("Binary process failed, OnError return: %v", err)
					break
				} else if binaryArgs == nil {
					return nil
				}
			}
			if err != nil {
				return err
			}
		}
		changed = true
		if doBackup {
			if err = atomic.ReplaceFile(path, helper.GetNewFilePath(tc.BackupDir, basename)); err != nil {
				return nil
			}
			return atomic.ReplaceFile(tempFilePath, targetFilePath)
		} else {
			if basename != targetFilename { // replace with a new name file
				if err = atomic.ReplaceFile(tempFilePath, targetFilePath); err != nil {
					return err
				}
				for _, suffix := range options.RenameAdditionalSuffixes {
					oldpath := path + suffix
					newpath := targetFilePath + suffix
					if util.FileExists(oldpath) && !util.FileExists(newpath) {
						if err := atomic.ReplaceFile(oldpath, newpath); err != nil {
							tc.Log("! failed to rename %q => %q: %v", oldpath, newpath, err)
						}
					}
				}
				return os.Remove(path)
			} else { // replace file in place
				os.Chmod(targetFilePath, 0600)
				return atomic.ReplaceFile(tempFilePath, targetFilePath)
			}
		}
	})
	return
}

func init() {
	transform.Register(&transform.Transformer{
		Name:   "executor",
		Action: Transformer,
	})
}
