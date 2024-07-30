package searchr

import (
	"fmt"
	"net/url"
	"os"
	"slices"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/cmd/common"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/site"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/helper"
)

var command = &cobra.Command{
	Use:     "searchr {site} {query-string}",
	Aliases: []string{"searchresource", "searchresources"},
	Short:   "searchr {site} {query-string}",
	Long:    `searchr.`,
	Args:    cobra.MatchAll(cobra.ExactArgs(2), cobra.OnlyValidArgs),
	RunE:    searchr,
}

var (
	cacheOnly          = false
	sum                = false
	add                = false
	addAsCompleted     = false
	addAsSkip          = false
	force              = false
	addMax             int
	max                int
	skip               int
	minResourceSizeStr string
	maxResourceSizeStr string
	checkPtoolSite     string
	sort               string
	order              string
)

func init() {
	command.Flags().BoolVarP(&cacheOnly, "cache", "", false, "Use cached index file only")
	command.Flags().BoolVarP(&sum, "sum", "", false, "Show summary only")
	command.Flags().BoolVarP(&force, "force", "f", false, "Force do action (Do NOT prompt for confirm)")
	command.Flags().BoolVarP(&add, "add", "", false, "Add resources to download queue")
	command.Flags().BoolVarP(&addAsCompleted, "add-as-completed", "", false, "Add resources to db, marked as completed")
	command.Flags().BoolVarP(&addAsSkip, "add-as-skip", "", false, "Add resources to db, marked as skip")
	command.Flags().IntVarP(&addMax, "add-max", "", -1, "Number limit of added resources to db. -1 == no limit")
	command.Flags().IntVarP(&max, "max", "", -1, "Number limit of displayed resources. -1 == no limit")
	command.Flags().IntVarP(&skip, "skip", "", 0, "Skip this number of resources from result")
	command.Flags().StringVarP(&checkPtoolSite, "check-ptool-site", "", "",
		"Ptool sitename. Prior adding, check ptool site for resource number, if same number resource already exists, "+
			"Add resource to db and mark as skip instead")
	command.Flags().StringVarP(&minResourceSizeStr, "min-resource-size", "", "-1",
		"Skip resource with size smaller than (<) this value. -1 == no limit")
	command.Flags().StringVarP(&maxResourceSizeStr, "max-resource-size", "", "-1",
		"Skip resource with size larger than (>) this value. -1 == no limit")
	cmd.AddEnumFlagP(command, &sort, "sort", "", common.ResourceSortFlag)
	cmd.AddEnumFlagP(command, &order, "order", "", common.OrderFlag)
	cmd.RootCmd.AddCommand(command)
}

func searchr(cmd *cobra.Command, args []string) (err error) {
	if util.CountNonZeroVariables(add, addAsCompleted, addAsSkip) > 1 {
		return fmt.Errorf("--add, --add-completed and --add-skip flags are NOT compatible")
	}
	var ptoolBinary string
	if checkPtoolSite != "" {
		if !add {
			return fmt.Errorf("--check-ptool-site must be used with --add flag")
		}
		if ptoolBinary, err = util.LookPathWithSelfDir("ptool"); err != nil {
			return fmt.Errorf("ptool binary not found: %w", err)
		}
	}
	minResourceSize, _ := util.RAMInBytes(minResourceSizeStr)
	maxResourceSize, _ := util.RAMInBytes(maxResourceSizeStr)
	sitename := args[0]
	qs := args[1]

	if qs == constants.NONE {
		qs = ""
	} else if strings.ContainsAny(qs, " \t\r\n") {
		qs = "raw=" + url.QueryEscape(qs)
	}
	if cacheOnly {
		if qs != "" {
			qs += "&"
		}
		qs += "cache=1"
	}

	siteInstance, err := site.CreateSite(sitename)
	if err != nil {
		return fmt.Errorf("failed to create site: %w", err)
	}
	resources, err := siteInstance.SearchResources(qs)
	if err != nil {
		return fmt.Errorf("failed to search site: %w", err)
	}
	if minResourceSize > 0 || maxResourceSize > 0 {
		resources = util.FilterSlice(resources, func(r site.Resource) bool {
			if minResourceSize > 0 && r.Size() < minResourceSize {
				return false
			}
			if maxResourceSize > 0 && r.Size() > maxResourceSize {
				return false
			}
			return true
		})
	}
	if sort != constants.NONE {
		less, more := -1, 1
		if order == "desc" {
			less, more = more, less
		}
		slices.SortStableFunc(resources, func(a, b site.Resource) int {
			switch sort {
			case "time":
				if a.Time() < b.Time() {
					return less
				} else if a.Time() > b.Time() {
					return more
				}
			case "size":
				if a.Size() < b.Size() {
					return less
				} else if a.Size() > b.Size() {
					return more
				}
			case "title":
				if a.Title() < b.Title() {
					return less
				} else if a.Title() > b.Title() {
					return more
				}
			case "author":
				if a.Author() < b.Author() {
					return less
				} else if a.Author() > b.Author() {
					return more
				}
			}
			return 0
		})
	}
	if skip > 0 {
		if skip <= len(resources) {
			resources = resources[skip:]
		} else {
			resources = []site.Resource{}
		}
	}
	if max > 0 && max < len(resources) {
		resources = resources[:max]
	}
	if !sum {
		resources.Print(os.Stdout)
	}
	fmt.Printf("Find %d resources (%s)\n", len(resources), util.BytesSize(float64(resources.Size())))

	if (add || addAsCompleted || addAsSkip) && len(resources) > 0 {
		added := 0
		addedSkip := 0
		addedCompleted := 0
		addStatus := ""
		if addAsCompleted {
			addStatus = "completed"
		} else if addAsSkip {
			addStatus = "skip"
		}
		if !force && !helper.AskYesNoConfirm(fmt.Sprintf("Add above %d resources to download db (set status: %q)",
			len(resources), addStatus)) {
			return fmt.Errorf("abort")
		}
		for _, resource := range resources {
			resourceStatus := addStatus
			identifier := siteInstance.GetIdentifier(resource.Id())
			existingResource := schema.ResourceDownload{}
			result := config.Db.First(&existingResource, "site = ? and identifier = ?", sitename, identifier)
			if result.Error == nil {
				log.Warnf("Resource %s (%s) already downloaded before. skip it\n", existingResource.Title, identifier)
				continue
			}
			if resource.Number() != "" && checkPtoolSite != "" {
				exists, err := helper.SearchPtoolSite(ptoolBinary, checkPtoolSite, resource.Number(), true, true)
				if err != nil {
					log.Warnf("Failed to search resource %s on ptool site: %v", resource.Number(), err)
					continue
				}
				if exists {
					log.Warnf("Resource %s already exists on ptool site, add it to db as skip", resource.Number())
					resourceStatus = "skip"
				}
			}
			resourceDownload := &schema.ResourceDownload{
				ResourceId: resource.Id(),
				Identifier: identifier,
				Site:       sitename,
				Number:     resource.Number(),
				Title:      resource.Title(),
				Author:     resource.Author(),
				Size:       resource.Size(),
				Tags:       resource.Tags(),
				Status:     resourceStatus,
			}
			if result := config.Db.Create(resourceDownload); result.Error != nil {
				log.Errorf("Failed to add resource %q to client: %v", resourceDownload.Title, result.Error)
				continue
			}
			switch resourceStatus {
			case "skip":
				addedSkip++
			case "completed":
				addedCompleted++
			case "":
				added++
			}
			if addMax > 0 && added >= addMax {
				break
			}
		}
		fmt.Fprintf(os.Stderr, "Done added %d resources to db. Added %d / %d marks of skip / completed to db\n",
			added, addedSkip, addedCompleted)
		if added > 0 {
			fmt.Fprintf(os.Stderr, "Use 'erodownloader watch' to start task queues.\n")
		}
	}
	return nil
}
