package get

import (
	"fmt"
	"net/url"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/site"
)

var command = &cobra.Command{
	Use:   "get {file-id}...",
	Short: "get {file-id}...",
	Long:  `get.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:  get,
}

var (
	showUrlOnly = false
)

func init() {
	command.Flags().BoolVarP(&showUrlOnly, "show-url-only", "", false, "Show only file raw download url")
	cmd.RootCmd.AddCommand(command)
}

func get(cmd *cobra.Command, args []string) (err error) {
	fileIds := args
	errorCnt := 0

	var files site.Files
	for _, id := range fileIds {
		sitename := ""
		if values, err := url.ParseQuery(id); err == nil {
			sitename = values.Get("site")
		}
		if sitename == "" {
			return fmt.Errorf("invalid id: no sitename")
		}
		siteInstance, err := site.CreateSite(sitename)
		if err != nil {
			return fmt.Errorf("failed to create site: %w", err)
		}
		file, err := siteInstance.GetFile(id)
		if err != nil {
			log.Errorf("Failed to get file %q: %v", id, err)
			errorCnt++
			continue
		}
		files = append(files, file)
	}
	if showUrlOnly {
		for _, file := range files {
			fmt.Printf("%s\n", file.RawUrl())
		}
		return nil
	}
	files.Print(os.Stdout)
	if errorCnt != 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
