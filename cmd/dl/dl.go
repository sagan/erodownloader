package dl

import (
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
)

// rootCmd represents the base command when called without any subcommands
var Command = &cobra.Command{
	Use:   "dl",
	Short: "Download manager",
	Long:  `Download manager`,
}

func init() {
	cmd.RootCmd.AddCommand(Command)
}
