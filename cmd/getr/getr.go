package getr

import (
	"fmt"
	"net/url"
	"os"

	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/site"
)

var command = &cobra.Command{
	Use:     "getr {resource-id}",
	Aliases: []string{"getresource"},
	Short:   "getresource {resource-id}",
	Long:    `getr.`,
	Args:    cobra.MatchAll(cobra.ExactArgs(1), cobra.OnlyValidArgs),
	RunE:    getr,
}

var (
	showIdOnly = false
)

func init() {
	command.Flags().BoolVarP(&showIdOnly, "show-id-only", "", false, "Show file ids only of resource")
	cmd.RootCmd.AddCommand(command)
}

func getr(cmd *cobra.Command, args []string) (err error) {
	sitename := ""
	id := args[0]
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
	files, err := siteInstance.GetResourceFiles(id)
	if err != nil {
		return fmt.Errorf("failed to get resource files: %w", err)
	}
	if showIdOnly {
		for _, file := range files {
			fmt.Printf("%s\n", file.Id())
		}
		return nil
	}
	files.Print(os.Stdout)
	return nil
}
