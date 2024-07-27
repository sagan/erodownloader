package delete

import (
	"fmt"

	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/cmd/dl"
)

var command = &cobra.Command{
	Use:     "delete {client} {download-id}...",
	Aliases: []string{"del"},
	Short:   "delete download tasks",
	Long:    `delete download tasks`,
	Args:    cobra.MatchAll(cobra.MinimumNArgs(2), cobra.OnlyValidArgs),
	RunE:    delete,
}

func init() {
	dl.Command.AddCommand(command)
}

func delete(cmd *cobra.Command, args []string) (err error) {
	clientname := args[0]
	ids := args[1:]
	errorCnt := 0

	clientInstance, err := client.CreateClient(clientname)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	for _, id := range ids {
		err := clientInstance.Delete(id)
		fmt.Printf("delete %s: err=%v\n", id, err)
		if err != nil {
			errorCnt++
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
