package search

import (
	"fmt"
	"os"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/site"
	"github.com/sagan/erodownloader/util/helper"
)

var command = &cobra.Command{
	Use:   "search {site} {query-string}",
	Short: "search {site} {query-string}",
	Long:  `search.`,
	Args:  cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE:  search,
}

var (
	add   = false
	force = false
)

func init() {
	command.Flags().BoolVarP(&force, "force", "f", false, "Force do action (Do NOT prompt for confirm)")
	command.Flags().BoolVarP(&add, "add", "", false, "Add files to download queue")
	cmd.RootCmd.AddCommand(command)
}

func search(cmd *cobra.Command, args []string) (err error) {
	sitename := args[0]
	qs := args[1]

	siteInstance, err := site.CreateSite(sitename)
	if err != nil {
		return fmt.Errorf("failed to create site: %w", err)
	}
	files, err := siteInstance.Search(qs)
	if err != nil {
		return fmt.Errorf("failed to search site: %w", err)
	}
	files.Print(os.Stdout)
	if add && len(files) > 0 {
		if !force && !helper.AskYesNoConfirm("Add above files to download queue") {
			return fmt.Errorf("abort")
		}
		for _, file := range files {
			if file.IsDir() {
				log.Warnf("File %q (%q) is dir. Do NOT add to client", file.Name(), file.Id())
				continue
			}
			identifier := siteInstance.GetIdentifier(file.Id())
			existingFile := schema.Download{}
			result := config.Db.First(&existingFile, "site = ? and identifier = ?", sitename, identifier)
			if result.Error == nil {
				log.Warnf("File %s (%s) already downloaded before. skip it\n", existingFile.Filename, identifier)
				continue
			}
			download := &schema.Download{
				FileId:     file.Id(),
				Filename:   file.Name(),
				Identifier: identifier,
				Site:       sitename,
			}
			if result := config.Db.Create(download); result.Error != nil {
				log.Errorf("Failed to add file %q to client: %v", download.Filename, result.Error)
			}
		}
		fmt.Fprintf(os.Stderr, "Done. Use 'erodownloader watch' to start task queues.\n")
	}
	return nil
}
