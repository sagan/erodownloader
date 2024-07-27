package strlen

import (
	"fmt"
	"unicode/utf8"

	"github.com/mattn/go-runewidth"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/util/stringutil"
)

var command = &cobra.Command{
	Use:   "strlen {string}...",
	Short: "strlen {string}...",
	Long:  `Display string length.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:  get,
}

func init() {
	cmd.RootCmd.AddCommand(command)
}

func get(cmd *cobra.Command, args []string) (err error) {
	strs := args

	for _, str := range strs {
		bytes := len(str)
		chars := utf8.RuneCountInString(str)
		width := runewidth.StringWidth(str)
		fmt.Printf("%q: %d bytes, %d chars, %d width\n", str, bytes, chars, width)
		if bytes > constants.FILENAME_MAX_LENGTH {
			fmt.Printf("Truncated to %d bytes: %q\n", constants.FILENAME_MAX_LENGTH,
				stringutil.StringPrefixInBytes(str, constants.FILENAME_MAX_LENGTH))
		}
		fmt.Printf("\n")
	}
	return nil
}
