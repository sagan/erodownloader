package pause

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/cmd/dl"
)

var command = &cobra.Command{
	Use:   "pause {client} {download-id}...",
	Short: "pause download tasks",
	Long:  `pause download tasks`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE:  pause,
}

func init() {
	dl.Command.AddCommand(command)
}

func pause(cmd *cobra.Command, args []string) (err error) {
	clientname := args[0]
	ids := args[1:]
	errorCnt := 0

	clientInstance, err := client.CreateClient(clientname)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	for _, id := range ids {
		err := clientInstance.Pause(id)
		fmt.Printf("pause %s: err=%v\n", id, err)
		if err != nil {
			errorCnt++
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
