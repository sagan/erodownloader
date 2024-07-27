package normalizename

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/helper"
)

var command = &cobra.Command{
	Use:   "normalizename {path}...",
	Short: "normalize file names inside pathes: truncate long names and replace restrictive chars",
	Long:  ``,
	RunE:  normalizename,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func normalizename(cmd *cobra.Command, args []string) (err error) {
	pathes := []string{}
	if len(args) == 0 {
		pathes = append(pathes, util.Unwrap(os.Getwd()))
	} else {
		for _, arg := range args {
			abspath, err := filepath.Abs(arg)
			if err != nil {
				return err
			}
			pathes = append(pathes, abspath)
		}
	}

	renamed, err := helper.NormalizeName(true, pathes...)
	fmt.Printf("Renamed %d files\n", renamed)
	return err
}
