package watch

import (
	"bufio"
	"fmt"
	"io"
	"os"
	"strings"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"
	"gorm.io/gorm"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/httpclient"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/helper"
	"github.com/sagan/erodownloader/web"
)

const DEFAULT_TIMEOUT = 1 // seconds
const MAX_TIMEOUT = 64    // seconds
const INTERVAL = 32
const MAX_DOWNLOADS = 4

var Command = &cobra.Command{
	Use:   "watch",
	Short: "watch and add add downloads continuously to client",
	Long:  ``,
	Args:  cobra.MatchAll(cobra.ExactArgs(0), cobra.OnlyValidArgs),
	RunE:  watch,
}

var (
	dryRun     = false
	stop       = false
	clientname = ""
)

func init() {
	Command.Flags().BoolVarP(&dryRun, "dry-run", "d", false, "Dry run")
	Command.Flags().BoolVarP(&stop, "stop", "", false, "Stop if all incoming downloads finished")
	Command.Flags().StringVarP(&clientname, "client", "", constants.LOCAL_CLIENT, "Used client name")
	cmd.RootCmd.AddCommand(Command)
}

func watch(cmd *cobra.Command, args []string) (err error) {
	clientInstance, err := client.CreateClient(clientname)
	if err != nil {
		return fmt.Errorf("failed to create client: %w", err)
	}
	timeout := DEFAULT_TIMEOUT
	clientTimeout := DEFAULT_TIMEOUT
	var failedCnt = map[string]int{} // file id => error count
	var download *schema.Download
	var resourceDownload *schema.ResourceDownload
	var clientDownloads client.Downloads
	var downloads []*schema.Download
	var resourceDownloads []*schema.ResourceDownload
	var result *gorm.DB
	db := config.Db

	go web.Start()
	go func() {
		for {
			in := bufio.NewReader(os.Stdin)
			result, err := in.ReadString('\n')
			if err != nil {
				log.Printf("stdin err: %v", err)
				return
			}
			if strings.HasPrefix(result, "r ") {
				httpclient.CloseHost(strings.TrimSpace(result[1:]))
			} else if result == "p" {
				PrintStatus(os.Stderr, clientname, db)
			} else if strings.HasPrefix(result, "reset") {
				db.Model(&schema.Download{}).Where("status = ?", "error").Updates(&schema.Download{
					Status: "downloading",
				})
			}
		}
	}()
	for {
		clientDownloads, err = clientInstance.GetAll()
		if checkErrorAndSleep(err, &clientTimeout, "get client torrents") {
			continue
		}
		downloadingCnt := 0
		errorCnt := 0

		// check every existing download in client and update db
		for _, clientDownload := range clientDownloads {
			if clientDownload.Status() == "downloading" {
				downloadingCnt++
				continue
			}
			newClientDownload, err := updateClientDownload(clientInstance, clientDownload, db, failedCnt, dryRun)
			if err != nil {
				log.Errorf("Failed to update client download %s: %v", clientDownload.Filename(), err)
				errorCnt++
				continue
			}
			if newClientDownload != nil {
				downloadingCnt++
			}
		}

		// find lost downlods (exists in db but does NOT exists in client) and re-create them in client
		clientDownloads, err = clientInstance.GetAll()
		if checkErrorAndSleep(err, &clientTimeout, "get client torrents") {
			continue
		}
		result = db.Find(&downloads, "client = ? and status = ?", clientname, "downloading")
		if result.Error != nil {
			log.Errorf("failed to find lost downloads: %v", result.Error)
		} else {
			for _, download := range downloads {
				if download.Status == "error" || download.DownloadId != "" && clientDownloads[download.DownloadId] != nil {
					continue
				}
				fmt.Fprintf(os.Stderr, "Re-create lost download task %s\n", download.Filename)
				if dryRun {
					continue
				}
				_, downloads, err := helper.AddDownloadTask(clientInstance, download.FileId, download.SavePath)
				handleAddFileError(db, failedCnt, download, err)
				if err != nil {
					continue
				}
				downloadingCnt++
				result = db.Model(download).Updates(map[string]any{
					"download_id": downloads[0].DownloadId,
					"status":      downloads[0].Status,
				})
				if result.Error != nil {
					log.Errorf("failed to update lost download new task id: %v", result.Error)
				}
			}
		}

		// check and update downloading resource status
		result = db.Find(&resourceDownloads, "client = ? and status = ?", clientname, "downloading")
		if result.Error != nil {
			log.Errorf("failed to get downloading resources: %v", result.Error)
		} else {
			for _, resourceDownload := range resourceDownloads {
				db.Transaction(func(tx *gorm.DB) error {
					var downloads []*schema.Download
					result := tx.Find(&downloads, "client = ? and resource_id = ?", clientname, resourceDownload.ResourceId)
					if result.Error != nil {
						log.Errorf("failed to find resource %s downloads: %v", resourceDownload.ResourceId, result.Error)
						return result.Error
					}
					isComplete := false
					isError := false
					msg := ""
					if len(downloads) == 0 {
						msg += "No file downloads task"
						isError = true
					} else {
						isComplete = true
						for _, download := range downloads {
							if download.Status == "error" {
								isError = true
								isComplete = false
								msg += fmt.Sprintf("file %s (%s) download error (%s); ",
									download.Filename, download.DownloadId, download.Note)
							}
							if download.Status != "completed" {
								isComplete = false
							}
						}
					}
					if isError {
						fmt.Fprintf(os.Stderr, "Resource %s download error\n", resourceDownload.Number)
						if dryRun {
							return nil
						}
						result = tx.Model(resourceDownload).Updates(&schema.ResourceDownload{
							Status: "error",
							Note:   "some file(s) of this resource failed to download: " + msg,
						})
						if result.Error != nil {
							return result.Error
						}
					} else if isComplete {
						fmt.Fprintf(os.Stderr, "Resource %s download completed (%s)\n",
							resourceDownload.Title, resourceDownload.SavePath)
						if dryRun {
							return nil
						}
						result = tx.Model(resourceDownload).Updates(&schema.ResourceDownload{
							Status: "completed",
						})
						if result.Error != nil {
							return result.Error
						}
						for _, download := range downloads {
							if download.DownloadId != "" {
								clientInstance.Delete(download.DownloadId)
							}
						}
					}
					return nil
				})
			}
		}

		// All tasks finished
		if stop && errorCnt == 0 &&
			db.First(&schema.ResourceDownload{}, "status = ? or status = ?", "downloading", "").Error ==
				gorm.ErrRecordNotFound &&
			db.First(&schema.Download{}, "status = ? or status = ?", "downloading", "").Error ==
				gorm.ErrRecordNotFound {
			fmt.Fprintf(os.Stderr, "All resources download completed\n")
			break
		}

		if downloadingCnt >= MAX_DOWNLOADS {
			sleepInterval("enough incoming downloads")
			continue
		}

		// add new file to client
		download = nil // Must reset dest before each query, or gorm will put current id in condition
		result = db.Order("updated_at DESC").First(&download, "status = ? and resource_id = ?", "", "")
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				log.Warnf("No new file to add")
			} else {
				log.Errorf("Failed to read new file: %v", result.Error)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Add new file download %s to client\n", download.Filename)
			if !dryRun {
				savePath := clientInstance.GetConfig().SavePath
				_, newDownloads, err := helper.AddDownloadTask(clientInstance, download.FileId, savePath)
				handleAddFileError(db, failedCnt, download, err)
				if checkErrorAndSleep(err, &timeout, "add new file to client") {
					continue
				}
				db.Transaction(func(tx *gorm.DB) error {
					if download.DownloadId != "" {
						clientInstance.Delete(download.DownloadId)
					}
					// insert new created task ids
					result = tx.Model(download).Updates(map[string]any{
						"save_path":   savePath,
						"status":      newDownloads[0].Status,
						"download_id": newDownloads[0].DownloadId,
						"client":      clientname,
					})
					if result.Error != nil {
						return result.Error
					}
					return nil
				})
			}
		}

		// add new resource to client
		resourceDownload = nil
		result = db.Order("failed asc, updated_at DESC").First(&resourceDownload, "status = ?", "")
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				log.Warnf("No new resource to add")
			} else {
				log.Errorf("Failed to read new resource: %v", result.Error)
			}
		} else {
			fmt.Fprintf(os.Stderr, "Add new resource download %s %s to client\n",
				resourceDownload.Number, resourceDownload.GetFilename())
			savePath := clientInstance.GetConfig().SavePath + client.Sep(clientInstance) + resourceDownload.GetFilename()
			if !dryRun {
				_, newDownloads, err := helper.AddDownloadTask(clientInstance, resourceDownload.ResourceId, savePath)
				handleAddResourceError(db, failedCnt, resourceDownload, err)
				if checkErrorAndSleep(err, &timeout, "add new resource to client") {
					continue
				}
				log.Tracef("%d files added to client", len(newDownloads))
				err = db.Transaction(func(tx *gorm.DB) error {
					// delete same resource downloading tasks from client and db
					var downloads []*schema.Download
					result := tx.Find(&downloads, "client = ? and resource_id = ?", clientname, resourceDownload.ResourceId)
					if result.Error != nil {
						return fmt.Errorf("failed to find existing resource tasks: %w", result.Error)
					}
					var clientDownloadIds []uint
					for _, download := range downloads {
						if download.DownloadId != "" {
							clientInstance.Delete(download.DownloadId)
						}
						clientDownloadIds = append(clientDownloadIds, download.ID)
					}
					if len(clientDownloadIds) > 0 {
						if result = tx.Delete(&schema.Download{}, clientDownloadIds); result.Error != nil {
							return fmt.Errorf("failed to delete existing resource tasks: %w", result.Error)
						}
					}
					// insert new created task ids
					result = tx.Model(resourceDownload).Updates(map[string]any{
						"save_path": savePath,
						"status":    "downloading",
						"client":    clientname,
					})
					if result.Error != nil {
						return fmt.Errorf("failed to update resource: %w", result.Error)
					}
					if result = tx.Create(&newDownloads); result.Error != nil {
						return fmt.Errorf("failed to create resource tasks: %w", result.Error)
					}
					return nil
				})
				if err != nil {
					log.Errorf("Failed to update db for added new resource : %v", err)
				}
			}
		}

		sleepInterval("All processes finished")
	}
	return nil
}

func updateClientDownload(clientInstance client.Client, clientDownload client.Download,
	db *gorm.DB, failedCnt map[string]int, dryRun bool) (newClientDownload client.Download, err error) {
	err = db.Transaction(func(tx *gorm.DB) error {
		var download *schema.Download
		result := tx.First(&download, " client = ? and download_id = ?",
			clientInstance.GetConfig().Name, clientDownload.Id())
		if result.Error != nil {
			if result.Error == gorm.ErrRecordNotFound {
				return nil
			}
			return result.Error
		}
		if download.Status == "error" || download.Status == "completed" {
			return nil
		}
		if clientDownload.Status() == "error" {
			tooManyFails := handleAddFileError(tx, failedCnt, download,
				fmt.Errorf("task %s download error: %s", clientDownload.Id(), clientDownload.Msg()))
			if tooManyFails {
				clientInstance.Delete(clientDownload.Id())
				return nil
			}
			log.Debugf("re-create error download task %s (%s) (failed: %d)",
				download.Filename, download.FileId, failedCnt[download.FileId])
			if dryRun {
				return nil
			}
			newClientDownloads, newDownloads, err := helper.AddDownloadTask(clientInstance, download.FileId,
				clientDownload.SavePath())
			handleAddFileError(tx, failedCnt, download, err)
			if err != nil {
				log.Debugf("re-create err download %s err=%v", download.Filename, err)
				return err
			}
			updates := map[string]any{
				"save_path":   newDownloads[0].SavePath,
				"client":      newDownloads[0].Client,
				"download_id": newDownloads[0].DownloadId,
				"status":      newDownloads[0].Status,
				"note":        "",
			}
			log.Tracef("re-created err download %s, new_download_id: %v", download.Filename, newClientDownloads[0].Id())
			clientInstance.Delete(clientDownload.Id())
			newClientDownload = newClientDownloads[0]
			if res := tx.Model(download).Updates(updates); res.Error != nil {
				return res.Error
			}
			return nil
		} else if clientDownload.Status() == "completed" {
			if dryRun {
				return nil
			}
			delete(failedCnt, download.FileId)
			if res := tx.Model(download).Updates(&schema.Download{Status: "completed"}); res.Error != nil {
				return res.Error
			}
			clientInstance.Delete(clientDownload.Id())
		}
		return nil
	})
	return newClientDownload, err
}

func handleAddResourceError(db *gorm.DB, failedCnt map[string]int, resourceDownload *schema.ResourceDownload, err error) (
	tooManyFails bool) {
	if err != nil {
		updates := map[string]any{
			"failed": resourceDownload.Failed + 1,
		}
		failedCnt[resourceDownload.ResourceId]++
		if failedCnt[resourceDownload.ResourceId] >= 10 {
			fmt.Fprintf(os.Stderr, "Resource %s download error due to too many fails\n", resourceDownload.Number)
			updates["status"] = "error"
			updates["note"] = fmt.Sprintf("Failed too many times. Last error: %s", err.Error())
		}
		res := db.Model(resourceDownload).Updates(updates)
		if res.Error != nil {
			return false
		}
		if failedCnt[resourceDownload.ResourceId] >= 10 {
			resourceDownload.Status = "error"
			delete(failedCnt, resourceDownload.ResourceId)
			return true
		}
	} else {
		failedCnt[resourceDownload.ResourceId] = 0
	}
	return false
}

func handleAddFileError(db *gorm.DB, failedCnt map[string]int, download *schema.Download,
	err error) (tooManyFails bool) {
	if err != nil {
		log.Errorf("failed to add file %q: %v", download.Filename, err)
		failedCnt[download.FileId]++
		if failedCnt[download.FileId] >= 10 {
			fmt.Fprintf(os.Stderr, "File %s download error due to too many fails\n", download.FileId)
			res := db.Model(download).Updates(map[string]any{
				"status": "error",
				"note":   fmt.Sprintf("Failed too many times. Last error: %s", err.Error()),
			})
			if res.Error != nil {
				log.Errorf("handleAddFileError failed to update db: %v", res.Error)
				return false
			}
			download.Status = "error"
			delete(failedCnt, download.FileId)
			return true
		}
	}
	return false
}

// If err is not nil, sleep in an exponential backoff way.
// Nil-err will reset the timeout to initial value.
// Return true if sleeped.
func checkErrorAndSleep(err error, timeout *int, action string) bool {
	if err != nil {
		log.Errorf("Sleep %ds due to action %q failed: %v", *timeout, action, err)
		util.Sleep(*timeout)
		*timeout = min(*timeout*2, MAX_TIMEOUT)
		return true
	} else if *timeout > DEFAULT_TIMEOUT {
		*timeout = DEFAULT_TIMEOUT
		return false
	}
	return false
}

func sleepInterval(tip string) {
	log.Warnf("Sleep %ds (%s)", INTERVAL, tip)
	util.Sleep(INTERVAL)
}

func PrintStatus(output io.Writer, clientname string, db *gorm.DB) {
	var downloads []*schema.Download
	var resourceDownloads []*schema.ResourceDownload

	db.Find(&downloads, "client = ? and status = ?", clientname, "downloading")
	schema.PrintDownloads(output, "Downloading files", downloads)
	fmt.Fprintf(output, "\n")

	db.Find(&resourceDownloads, "client = ? and status = ?", clientname, "completed")
	schema.PrintResourceDownloads(output, "Completed downloaded resources", resourceDownloads)
	fmt.Fprintf(output, "\n")

	db.Find(&downloads, "client = ? and status = ? and resource_id = ?", clientname, "completed", "")
	schema.PrintDownloads(output, "Completed downloaded files", downloads)
	fmt.Fprintf(output, "\n")

	db.Find(&resourceDownloads, "status = ?", "")
	schema.PrintResourceDownloads(output, "Queued resources", resourceDownloads)
	fmt.Fprintf(output, "\n")

	db.Find(&downloads, "status = ? and resource_id = ?", "", "")
	schema.PrintDownloads(output, "Queued files", downloads)
	fmt.Fprintf(output, "\n")
}
