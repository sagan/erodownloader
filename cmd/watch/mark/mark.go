package mark

import (
	"fmt"

	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/sagan/erodownloader/cmd/watch"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/util/helper"
)

var command = &cobra.Command{
	Use:   "mark",
	Short: "mark resources / files in db",
	Long:  ``,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  mark,
}

var (
	checkSkip      bool
	maxTries       int
	checkPtoolSite string
)

func init() {
	command.Flags().IntVarP(&maxTries, "max-tries", "", 3, "Max tries number for each record. -1 = unlimited")
	command.Flags().BoolVarP(&checkSkip, "check-skip", "", false,
		"Check skip resources, remove skip mark if site torrent does not exist")
	command.Flags().StringVarP(&checkPtoolSite, "check-ptool-site", "", "",
		"Ptool sitename. Check ptool site for resource number, if same number resource already exists, "+
			"mark error / queued resource as skip")
	watch.Command.AddCommand(command)
}

func mark(cmd *cobra.Command, args []string) (err error) {
	if checkPtoolSite == "" {
		return nil
	}

	db := config.Db
	var resourceDownloads []*schema.ResourceDownload
	if checkSkip {
		db.Find(&resourceDownloads, "status = ? or status = ? or status = ?", "error", "skip", "")
	} else {
		db.Find(&resourceDownloads, "status = ? or status = ?", "error", "")
	}

	if len(resourceDownloads) == 0 {
		fmt.Printf("No db resources\n")
		return nil
	}

	errorCnt := 0
mainloop:
	for i, resourceDownload := range resourceDownloads {
		fmt.Printf("(%d/%d) ", i+1, len(resourceDownloads))
		if resourceDownload.Number == "" {
			fmt.Printf("- %q : no number\n", resourceDownload.ResourceId)
			continue
		}
		tries := 0
		var exists bool
		for {
			tries++
			exists, err = helper.SearchPtoolSite(checkPtoolSite, resourceDownload.Number, true, true)
			if err == nil {
				break
			}
			fmt.Printf("X %q : failed to search ptool site: %v (tries %d)\n", resourceDownload.Number, err, tries)
			if maxTries >= 0 && tries >= maxTries {
				errorCnt++
				continue mainloop
			}
		}
		if resourceDownload.Status == "skip" {
			if !exists {
				db.Model(resourceDownload).Updates(map[string]any{
					"status": "",
				})
				fmt.Printf("= %q : does not exists on ptool site, remove skip mark\n", resourceDownload.Number)
			} else {
				fmt.Printf(". %q : already skip\n", resourceDownload.Number)
			}
			continue
		}
		fmt.Printf(". %q : does not exists on ptool site\n", resourceDownload.Number)
		err = db.Transaction(func(tx *gorm.DB) error {
			res := tx.Where("resource_id = ?", resourceDownload.ResourceId).Delete(&schema.Download{})
			if res.Error != nil {
				return res.Error
			}
			res = tx.Model(&schema.ResourceDownload{}).Where("id = ?", resourceDownload.ID).Updates(map[string]any{
				"status": "skip",
			})
			return res.Error
		})
		if err != nil {
			fmt.Printf("X %q : exists on ptool site, failed to mark it as skip: %v\n", resourceDownload.Number, err)
			errorCnt++
		} else {
			fmt.Printf("! %q : exists on ptool site, mark it as skip\n", resourceDownload.Number)
		}
	}
	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
