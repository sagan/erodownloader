package status

import (
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd/watch"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/constants"
)

var command = &cobra.Command{
	Use:   "status",
	Short: "show status",
	Long:  ``,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  status,
}

var (
	clientname = ""
)

func init() {
	command.Flags().StringVarP(&clientname, "client", "", constants.LOCAL_CLIENT, "Used client name")
	watch.Command.AddCommand(command)
}

func status(cmd *cobra.Command, args []string) (err error) {
	watch.PrintStatus(os.Stderr, clientname, config.Db)
	return nil
}
