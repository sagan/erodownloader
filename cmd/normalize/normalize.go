package normalize

import (
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"runtime"
	"slices"
	"strings"
	"time"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/helper"
	"github.com/sagan/erodownloader/util/stringutil"
)

var command = &cobra.Command{
	Use:   "normalize [--save-path {save-path}] [content-path]...",
	Short: "normalize",
	Long:  `normalize.`,
	RunE:  normalize,
}

var (
	all                    = false
	noFlac                 = false
	doClean                = false
	doRestore              = false
	force                  = false
	lock                   = false
	noSkipRecentlyModified = false
	savePath               = ""
	moveTo                 = ""
	passwords              []string
	options                []string
)

func init() {
	command.Flags().BoolVarP(&all, "all", "a", false, `Process all. Equivalent to "--no-skip-recently-modified"`)
	command.Flags().BoolVarP(&noSkipRecentlyModified, "no-skip-recently-modified", "", false,
		"Process (do not skip) recently modified dirs also")
	command.Flags().BoolVarP(&doClean, "clean", "", false, "Clean backup files and tmp files")
	command.Flags().BoolVarP(&doRestore, "restore", "", false, `Restore "`+
		transform.TF_PREFIX+`.*" content dir names to original`)
	command.Flags().BoolVarP(&noFlac, "no-flac", "", false, "Disable flac normalizer (convert wav to flac)")
	command.Flags().BoolVarP(&force, "force", "f", false, "Force do action (Do NOT prompt for confirm)")
	command.Flags().BoolVarP(&lock, "lock", "l", false,
		"Lock each content-dir (prepend the '"+transform.TF_PREFIX+"' prefix to it's filename) when processing")
	command.Flags().StringVarP(&savePath, "save-path", "", "", "Process all folders of this path dir")
	command.Flags().StringVarP(&moveTo, "move-to", "", "", "Move successfully processed content-dir to this folder")
	command.Flags().StringArrayVarP(&options, "option", "o", nil, `Set transformer(s) option(s). E.g. "foo=bar"`)
	command.Flags().StringArrayVarP(&passwords, "password", "p", nil,
		`Set password(s) for rar / 7z file. Equivalent to "--option password=PASSWORD"`)
	cmd.RootCmd.AddCommand(command)
}

func normalize(cmd *cobra.Command, args []string) (err error) {
	if all {
		noSkipRecentlyModified = true
	}
	if doClean && doRestore {
		return fmt.Errorf("--clean and --restore flags are NOT compatible")
	}
	var optionValues = url.Values{}
	for _, option := range options {
		values, err := url.ParseQuery(option)
		if err != nil {
			return fmt.Errorf("invalid option %v", option)
		}
		for k, v := range values {
			optionValues[k] = v
		}
	}
	for _, password := range passwords {
		optionValues.Add("password", password)
	}
	for _, password := range config.Data.Passwords {
		optionValues.Add("password", password)
	}
	if optionValues.Has("password") {
		optionValues["password"] = util.UniqueSlice(optionValues["password"])
	}
	if savePath != "" {
		optionValues.Set("bakdir", filepath.Join(savePath, transform.BAK_DIR))
	}
	if savePath != "" && len(args) > 0 {
		return fmt.Errorf("--save-path arg can not be combined with other positional args")
	}
	var dirs []string // each dir is absolute path
	if savePath != "" {
		entries, err := os.ReadDir(savePath)
		if err != nil {
			return fmt.Errorf("failed to read dir: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() ||
				doRestore && !strings.HasPrefix(entry.Name(), transform.TF_PREFIX) ||
				!doRestore && strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			dir, err := filepath.Abs(filepath.Join(savePath, entry.Name()))
			if err != nil {
				return fmt.Errorf("failed to get abs dir of %q in save path", entry.Name())
			}
			dirs = append(dirs, dir)
		}
	} else if len(args) == 0 {
		dirs = []string{util.Unwrap(os.Getwd())}
	} else {
		for _, arg := range args {
			dir, err := filepath.Abs(arg)
			if err != nil {
				return err
			}
			stat, err := os.Stat(dir)
			if err != nil || !stat.IsDir() {
				return fmt.Errorf("%q access denied or is not dir (err=%v)", dir, err)
			}
			dirs = append(dirs, dir)
		}
	}
	if doClean {
		return clean(dirs)
	} else if doRestore {
		return restore(dirs)
	}
	if moveTo != "" {
		if err = os.MkdirAll(moveTo, 0700); err != nil {
			return fmt.Errorf("failed to make move-to dir: %w", err)
		}
	}

	if !optionValues.Has("sevenzip_binary") {
		if binpath, err := util.LookPathWithSelfDir("7z"); err == nil {
			optionValues.Set("sevenzip_binary", binpath)
		} else {
			searchPathes := []string{}
			if runtime.GOOS == "windows" {
				if env := os.Getenv("ProgramFiles"); env != "" {
					searchPathes = append(searchPathes, filepath.Join(env, "7-Zip"))
				}
				if env := os.Getenv("ProgramFiles(x86)"); env != "" {
					searchPathes = append(searchPathes, filepath.Join(env, "7-Zip"))
				}
			}
			for _, searchPath := range searchPathes {
				binpath := filepath.Join(searchPath, "7z")
				if runtime.GOOS == "windows" {
					binpath += ".exe"
				}
				if util.FileExists(binpath) {
					optionValues.Set("sevenzip_binary", binpath)
				}
			}
		}
	}
	if !optionValues.Has("sevenzip_binary") {
		log.Warnf("7z binary is not found. " +
			"Consider install 7-Zip and add it's binary path to PATH for better handling of archives.")
	} else {
		log.Debugf("sevenzip_binary: %s", optionValues.Get("sevenzip_binary"))
	}

	errorCnt := 0
	transformers := []any{[]string{"decensorship", "correctext", "decompress", "text", "nocredit", "denesting"}, -1}
	if !noFlac && !optionValues.Has("flac_binary") {
		binpath, err := util.LookPathWithSelfDir("flac")
		if err != nil {
			log.Fatalf(`flac binary not found, please add "flac" binary to PATH`)
		}
		optionValues.Set("flac_binary", binpath)
		transformers = append(transformers, "wav")
	}
	transformers = append(transformers, "noempty", "normalizename")

	transformers = append(transformers, "clean")
	log.Warnf("Used transformers: %v", transformers)
	normalizer, err := transform.NewNormalizer(transformers...)
	if err != nil {
		return fmt.Errorf("fail to create normalizer: %w", err)
	}
	// each dir is a absolute full path.
	for i, dir := range dirs {
		fmt.Printf("(%d/%d) ", i+1, len(dirs))
		if stat, err := os.Stat(dir); err != nil {
			fmt.Printf("X %q: faied to access dir: %v\n", dir, err)
			errorCnt++
			continue
		} else if now := time.Now().Unix(); !noSkipRecentlyModified &&
			stat.ModTime().Unix() <= now && (now-stat.ModTime().Unix() <= 60) {
			fmt.Printf("- %q: skip recently modified dir\n", dir)
			continue
		} else if entries, err := os.ReadDir(dir); err != nil {
			fmt.Printf("X %q: failed to read dir: %v\n", dir, err)
			errorCnt++
			continue
		} else if len(entries) == 0 {
			fmt.Printf("- %q: skip empty dir\n", dir)
			continue
		} else if slices.ContainsFunc(entries, func(entry fs.DirEntry) bool {
			return !entry.IsDir() && stringutil.HasAnySuffix(entry.Name(), constants.IncompleteFileExts...)
		}) {
			fmt.Printf("- %q: skip incomplete dir\n", dir)
			continue
		}
		fmt.Printf("→ %q: processing.\n", dir)
		base := filepath.Base(dir)
		processpath := dir
		if lock {
			tmppath := filepath.Join(filepath.Dir(dir), transform.TF_PREFIX+base)
			if err := atomic.ReplaceFile(dir, tmppath); err != nil {
				fmt.Printf("X %q: failed to lock (rename to %q): %v", dir, tmppath, err)
				errorCnt++
				continue
			}
			processpath = tmppath
		}
		tc := normalizer.Transform(processpath, optionValues)
		targetpath := dir // after success processed, final path of dir.
		if tc.Err != nil {
			if errors.Is(tc.Err, transform.ErrInvalid) {
				fmt.Printf("! %q: invalid contents, changed=%t, bak_dir=%s.\n", dir, tc.Changed, tc.BackupDir)
			} else {
				fmt.Printf("X %q: changed=%t, err=%v, bak_dir=%s.\n", dir, tc.Changed, tc.Err, tc.BackupDir)
				errorCnt++
			}
			if processpath != dir && !util.FileExists(dir) {
				atomic.ReplaceFile(processpath, dir)
			}
			continue
		} else {
			if tc.Changed {
				fmt.Printf("✓ %q: bak_dir=%s\n", dir, tc.BackupDir)
			} else {
				fmt.Printf("- %q: no_changes.\n", dir)
			}
			if moveTo != "" {
				targetpath = filepath.Join(moveTo, base)
			}
		}
		if targetpath != processpath {
			if util.FileExists(targetpath) {
				log.Errorf("Normalize %q final: failed to rename to %q: target already exists", processpath, targetpath)
				if dir != processpath && !util.FileExists(dir) {
					atomic.ReplaceFile(processpath, dir)
				}
			} else if err := atomic.ReplaceFile(processpath, targetpath); err != nil {
				log.Errorf("Normalize %q final: failed to rename to %q: %v", processpath, targetpath, err)
			}
		}
	}
	fmt.Printf("\nAll Done with %d errors. Logs can be found in '%q' of bak dir(s)\n", errorCnt, transform.LOG_FILE)
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}

func clean(dirs []string) (err error) {
	var delfiles []string
	var tmpfiles = []string{transform.BAK_DIR, transform.TMP_DIR}
	for _, dir := range dirs {
		for _, tmpfile := range tmpfiles {
			if file := filepath.Join(dir, tmpfile); util.FileExists(file) {
				delfiles = append(delfiles, file)
			}
		}
	}
	if savePath != "" {
		if file := filepath.Join(savePath, transform.BAK_DIR); util.FileExists(file) {
			delfiles = append(delfiles, file)
		}
	}
	if len(delfiles) == 0 {
		log.Infof("No files to clean")
		return nil
	}
	if !force {
		fmt.Printf("\n")
		for _, delfile := range delfiles {
			fmt.Printf("%s\n", delfile)
		}
		fmt.Printf("\n")
		if !helper.AskYesNoConfirm("Above backup / tmp dirs / files will be removed") {
			return fmt.Errorf("abort")
		}
	}
	errorCnt := 0
	for _, delfile := range delfiles {
		if err := os.RemoveAll(delfile); err != nil {
			log.Errorf("Failed to remove %q: %v", delfile, err)
			errorCnt++
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}

func restore(dirs []string) (err error) {
	errorCnt := 0
	for _, dir := range dirs {
		dirname := filepath.Dir(dir)
		basename := filepath.Base(dir)
		if !strings.HasPrefix(basename, transform.TF_PREFIX) && len(basename) > len(transform.TF_PREFIX) {
			continue
		}
		newdir := filepath.Join(dirname, basename[len(transform.TF_PREFIX):])
		if util.FileExists(newdir) {
			fmt.Printf("X %q => %q: target already exists", dir, newdir)
		} else if err := atomic.ReplaceFile(dir, newdir); err != nil {
			fmt.Printf("X %q => %q: %v", dir, newdir, err)
		} else {
			fmt.Printf("✓ %q => %q", dir, newdir)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
