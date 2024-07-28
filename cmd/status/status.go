package status

import (
	"fmt"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/cmd"
)

var command = &cobra.Command{
	Use:   "status {client}",
	Short: "status {client}",
	Long:  `status {client}`,
	Args:  cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:  get,
}

var (
	showUrlOnly = false
)

func init() {
	cmd.RootCmd.AddCommand(command)
}

func get(cmd *cobra.Command, args []string) (err error) {
	clientname := args[0]

	clientInstance, err := client.CreateClient(clientname)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	clientStatus, err := clientInstance.GetStatus()
	if err != nil {
		return fmt.Errorf("failed to get client status: %w", err)
	}
	client.PrintStatus(os.Stdout, clientInstance, clientStatus)
	return nil
}
