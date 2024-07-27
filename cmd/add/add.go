package add

import (
	"fmt"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/util/helper"
)

var command = &cobra.Command{
	Use:   "add {resource-id | file-id}...",
	Short: "add resource or file to client to download",
	Long:  ``,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:  add,
}

var (
	savepath   string
	clientname string
)

func init() {
	command.Flags().StringVarP(&savepath, "save-path", "", "", "Manually set save path")
	command.Flags().StringVarP(&clientname, "client", "", constants.LOCAL_CLIENT, "Used client name")
	cmd.RootCmd.AddCommand(command)
}

func add(cmd *cobra.Command, args []string) (err error) {
	ids := args
	erorCnt := 0
	clientInstance, err := client.CreateClient(clientname)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	if _, err := clientInstance.GetStatus(); err != nil {
		return fmt.Errorf("failed to connect to client: %w", err)
	}
	if savepath == "" {
		savepath = clientInstance.GetConfig().SavePath
	}

	for _, id := range ids {
		_, _, err := helper.AddDownloadTask(clientInstance, id, savepath)
		log.Infof("add file %q: err=%v", id, err)
		if err != nil {
			erorCnt++
		}
	}
	return nil
}
