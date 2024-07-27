package scrape

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	"github.com/natefinch/atomic"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/scraper"
	"github.com/sagan/erodownloader/util"
)

var command = &cobra.Command{
	Use:   "scrape [--save-path {save-path}] [content-path]",
	Short: "scrape",
	Long:  ``,
	RunE:  add,
}

var (
	force        = false
	noRename     = false
	moveTo       string
	savePath     string
	scraperNames string
)

func init() {
	command.Flags().BoolVarP(&force, "force", "f", false, "Force re-generate")
	command.Flags().BoolVarP(&noRename, "no-rename", "", false, "Do not allow renaming content dir")
	command.Flags().StringVarP(&savePath, "save-path", "", "", "Process all folders of this path dir")
	command.Flags().StringVarP(&scraperNames, "scraper", "", "dlsite,asmrone,hvdb,dmm",
		"Comma-seperated used scraper names")
	command.Flags().StringVarP(&moveTo, "move-to", "", "", "Move successfully scraped content-dir to this folder")
	cmd.RootCmd.AddCommand(command)
}

func add(cmd *cobra.Command, args []string) (err error) {
	if moveTo != "" {
		if err = os.MkdirAll(moveTo, 0700); err != nil {
			return fmt.Errorf("failed to make move-to dir: %w", err)
		}
	}
	if savePath != "" && len(args) > 0 {
		return fmt.Errorf("--save-path arg can not be combined with other positional args")
	}
	var tmpdir string
	var dirs []string // each dir is absolute path
	if savePath != "" {
		tmpdir := filepath.Join(savePath, constants.TMP_DIR)
		if err = util.MakeCleanTmpDir(tmpdir); err != nil {
			return fmt.Errorf("failed to create tmp dir: %w", err)
		}
		defer os.RemoveAll(tmpdir)

		entries, err := os.ReadDir(savePath)
		if err != nil {
			return fmt.Errorf("failed to read dir: %w", err)
		}
		for _, entry := range entries {
			if !entry.IsDir() || strings.HasPrefix(entry.Name(), ".") {
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

	scrapers, err := scraper.NewScrapers(util.SplitCsv(scraperNames)...)
	if err != nil {
		return fmt.Errorf("failed to create scrapers: %w", err)
	}
	errorCnt := 0
	for i, dir := range dirs {
		fmt.Printf("(%d/%d) ", i+1, len(dirs))
		dirname := filepath.Dir(dir)
		basename := filepath.Base(dir)
		localtmpdir := tmpdir
		if tmpdir == "" {
			localtmpdir = filepath.Join(dir, constants.TMP_DIR)
			if err = util.MakeCleanTmpDir(localtmpdir); err != nil {
				fmt.Printf("X %q: failed to create tmp dir: %v\n", dir, err)
				errorCnt++
				continue
			}
		}

		metadata, err := scrapers.Scrape(dir, localtmpdir, force)
		if tmpdir == "" {
			os.RemoveAll(localtmpdir)
		}
		if err == nil {
			targetpath := dirname
			if moveTo != "" {
				targetpath = moveTo
			}
			if !noRename && metadata.ShouldRename {
				newname := metadata.GetCanonicalFilename()
				targetpath = filepath.Join(targetpath, newname)
			} else {
				targetpath = filepath.Join(targetpath, basename)
			}
			renameTip := ""
			if dir != targetpath {
				if util.FileExists(targetpath) {
					renameTip = fmt.Sprintf("rename target %q already exists", targetpath)
				} else if err := atomic.ReplaceFile(dir, targetpath); err != nil {
					renameTip = fmt.Sprintf("rename to %q failed: %v", targetpath, err)
				} else {
					renameTip = fmt.Sprintf("renamed to %q", targetpath)
				}
			}
			if renameTip != "" {
				renameTip = fmt.Sprintf(" (%s)", renameTip)
			}
			fmt.Printf("✓ %q: scrapped (%s)%s\n", dir, metadata.GeneratedBy, renameTip)
		} else if err == scraper.ErrExists {
			fmt.Printf("- %q: metadata exists (already scrapped before)\n", dir)
		} else {
			fmt.Printf("X %q: failed to scrap: %v\n", dir, err)
			errorCnt++
		}
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
