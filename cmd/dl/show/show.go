package show

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/cmd/dl"
)

var command = &cobra.Command{
	Use:   "show {client} [id]...",
	Short: "show {client} [id]...",
	Long:  `show {client}`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:  show,
}

var (
	filter = ""
)

func init() {
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter download tasks by keyword")
	dl.Command.AddCommand(command)
}

func show(cmd *cobra.Command, args []string) (err error) {
	clientname := args[0]
	ids := args[1:]
	clientInstance, err := client.CreateClient(clientname)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	allDownloads, err := clientInstance.GetAll()
	if err != nil {
		return fmt.Errorf("failed to get downloads: %w", err)
	}
	if len(ids) == 0 {
		allDownloads.Print(os.Stdout, filter)
	} else {
		for _, id := range ids {
			if allDownloads[id] == nil {
				log.Errorf("download task %s not found", id)
				continue
			}
			if filter != "" && !client.MatchDownloadWitchFilter(allDownloads[id], filter) {
				continue
			}
			client.PrintDownload(os.Stdout, allDownloads[id], 0)
		}
	}
	return nil
}
