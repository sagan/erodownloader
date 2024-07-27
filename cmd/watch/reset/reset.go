package reset

import (
	"fmt"
	"net/url"
	"os"
	"strings"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/sagan/erodownloader/cmd/watch"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/site"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/helper"
)

var command = &cobra.Command{
	Use:   "reset",
	Short: "reset error resources / files in db",
	Long:  ``,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  reseterror,
}

var (
	force  bool
	readd  bool
	all    bool
	sum    bool
	filter string
)

func init() {
	command.Flags().BoolVarP(&sum, "sum", "", false, "Show summary only")
	command.Flags().BoolVarP(&force, "force", "f", false, "Force. Do without prompt")
	command.Flags().BoolVarP(&readd, "readd", "", false, "Re-add error tasks from original site")
	command.Flags().BoolVarP(&all, "all", "a", false, "Reset all tasks (including queued tasks)")
	command.Flags().StringVarP(&filter, "filter", "", "", "Filter download / resource download")
	watch.Command.AddCommand(command)
}

func reseterror(cmd *cobra.Command, args []string) (err error) {
	db := config.Db
	var downloads []*schema.Download
	var resourceDownloads []*schema.ResourceDownload

	if all {
		db.Find(&downloads, "resource_id = ? and (status = ? or status = ?)", "", "error", "")
		db.Find(&resourceDownloads, "status = ? or status = ?", "error", "")
	} else {
		db.Find(&downloads, "status = ? and resource_id = ?", "error", "")
		db.Find(&resourceDownloads, "status = ?", "error")
	}
	if filter != "" {
		downloads = util.FilterSlice(downloads, func(d *schema.Download) bool {
			return strings.Contains(d.Filename, filter)
		})
		resourceDownloads = util.FilterSlice(resourceDownloads, func(rd *schema.ResourceDownload) bool {
			return strings.Contains(rd.GetFilename(), filter)
		})
	}

	if len(downloads) == 0 && len(resourceDownloads) == 0 {
		fmt.Printf("No matched downloads / resource_downloads\n")
		return nil
	}

	if !force {
		if !sum {
			fmt.Printf("Error file downloads:\n")
			schema.PrintDownloads(os.Stdout, "Downloading files", downloads)
			fmt.Printf("\n")

			fmt.Printf("Error resource downloads:\n")
			schema.PrintResourceDownloads(os.Stdout, "Downloading resources", resourceDownloads)
			fmt.Printf("\n")
		}
		if !helper.AskYesNoConfirm(fmt.Sprintf("Reset above %d download / %d resource tasks",
			len(downloads), len(resourceDownloads))) {
			return fmt.Errorf("abort")
		}
	}

	res1 := db.Model(&schema.Download{}).Where("status = ?", "error").Updates(map[string]any{
		"status": "",
	})
	if !readd {
		res2 := db.Model(&schema.ResourceDownload{}).Where("status = ?", "error").Updates(map[string]any{
			"failed": 0,
			"status": "",
		})
		fmt.Printf("Resetted error tasks: %d file downloads, %d resource downloads\n",
			res1.RowsAffected, res2.RowsAffected)
		return nil
	}

	fmt.Printf("Resetted error tasks: %d file downloads\n", res1.RowsAffected)
	fmt.Printf("Re-adding resource downloads\n")

	errorCnt := 0
	for _, resourceDownload := range resourceDownloads {
		if resourceDownload.Number == "" {
			fmt.Printf("X %q: failed to readd, no number", resourceDownload.ResourceId)
			errorCnt++
			continue
		}
		sitename := resourceDownload.Site
		siteInstance, err := site.CreateSite(sitename)
		if err != nil {
			fmt.Printf("X %q: failed to create site: %v", resourceDownload.Identifier, err)
			errorCnt++
			continue
		}
		resources, err := siteInstance.SearchResources("number=" + url.QueryEscape(resourceDownload.Number))
		if err != nil {
			fmt.Printf("X %q: failed to search number = %q resource: %v",
				resourceDownload.Identifier, resourceDownload.Number, err)
			errorCnt++
			continue
		}
		if len(resources) == 0 {
			fmt.Printf("X %q: resource no longer exists in site", resourceDownload.Identifier)
			errorCnt++
			continue
		}
		resource := resources[0]
		identifier := siteInstance.GetIdentifier(resource.Id())
		err = db.Transaction(func(tx *gorm.DB) error {
			res := tx.Delete(&schema.ResourceDownload{}, resourceDownload.ID)
			if res.Error != nil {
				return res.Error
			}
			res = tx.Where("resource_id = ?", resourceDownload.ResourceId).Delete(&schema.Download{})
			if res.Error != nil {
				return res.Error
			}
			resourceDownload = &schema.ResourceDownload{
				ResourceId: resource.Id(),
				Identifier: identifier,
				Site:       sitename,
				Number:     resource.Number(),
				Title:      resource.Title(),
				Author:     resource.Author(),
				Size:       resource.Size(),
				Tags:       resource.Tags(),
				Status:     "",
			}
			res = tx.Create(resourceDownload)
			return res.Error
		})
		if err != nil {
			fmt.Printf("Failed to re-added %q resource: %v\n", identifier, err)
			errorCnt++
		} else {
			fmt.Printf("Re-added %q resource\n", identifier)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
