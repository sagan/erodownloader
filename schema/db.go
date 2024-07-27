package schema

import (
	"fmt"
	"io"
	"strings"
	"time"

	"github.com/glebarez/sqlite"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/stringutil"
	"gorm.io/gorm"
	"gorm.io/gorm/logger"
)

const NAME_WIDTH = 30

// Client, Status, TaskId, Note
const FORMAT = "  %-10s  %-10s  %-16s  %s\n"

// Do NOT embed gorm.Model as we need to set json tag
type ResourceDownload struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	ResourceId string    `json:"resource_id"`
	Identifier string    `json:"identifier"`
	Site       string    `json:"site"`
	Status     string    `json:"status"`
	Size       int64     `json:"size"`
	Number     string    `json:"number"`
	Title      string    `json:"title"`
	Author     string    `json:"author"`
	Client     string    `json:"client"`
	SavePath   string    `json:"save_path"`
	Note       string    `json:"note"`
	Failed     int       `json:"failed"` // failed times count
	Tags       Tags      `gorm:"type:string" json:"tags"`
}

// a download task in client.
// For convenience, it implements client.DownloadTask.
type Download struct {
	ID         uint      `gorm:"primarykey" json:"id"`
	CreatedAt  time.Time `json:"created_at"`
	UpdatedAt  time.Time `json:"updated_at"`
	DownloadId string    `json:"download_id"` // download task id in client
	FileId     string    `json:"file_id"`
	Identifier string    `json:"identifier"`
	Site       string    `json:"site"`
	FileUrl    string    `json:"file_url"`
	Filename   string    `json:"filename"`
	SavePath   string    `json:"save_path"`
	ResourceId string    `json:"resource_id"`
	Status     string    `json:"status"` // (empty)|downloading|completed|error
	Note       string    `json:"note"`
	Client     string    `json:"client"`
}

// Return suitable folder name
func (r *ResourceDownload) GetFilename() (filename string) {
	author := util.CleanBasenameComponent(r.Author)
	title := util.CleanBasenameComponent(r.Title)
	filename += fmt.Sprintf("[%s]", r.Number)
	if author != "" {
		filename += fmt.Sprintf("[%s]", author)
	}
	if title != "" {
		filename += title
	}
	return util.CleanBasename(filename)
}

func (d *Download) GetFilename() string {
	return d.Filename
}

func (d *Download) GetPaused() bool {
	return d.Status == "paused"
}

func (d *Download) GetSavePath() string {
	return d.SavePath
}

func (d *Download) GetUrl() string {
	return d.FileUrl
}

func PrintDownloads(output io.Writer, title string, downloads []*Download) {
	fmt.Fprintf(output, "%s (%d):\n", title, len(downloads))
	fmt.Fprintf(output, "%-*s", NAME_WIDTH, "Filename")
	fmt.Fprintf(output, FORMAT, "Client", "Status", "TaskId", "Note")
	for _, download := range downloads {
		var notes []string
		if download.ResourceId != "" {
			notes = append(notes, "resource:"+download.ResourceId)
		} else if download.FileId != "" {
			notes = append(notes, "file:"+download.FileId)
		}
		if download.SavePath != "" {
			notes = append(notes, download.SavePath)
		}
		stringutil.PrintStringInWidth(output, download.Filename, NAME_WIDTH, true)
		fmt.Fprintf(output, FORMAT, download.Client, download.Status, download.DownloadId, strings.Join(notes, " ; "))
	}
}

func PrintResourceDownloads(output io.Writer, title string, resourceDownloads []*ResourceDownload) {
	fmt.Fprintf(output, "%s (%d):\n", title, len(resourceDownloads))
	fmt.Fprintf(output, "%-*s", NAME_WIDTH, "Name")
	fmt.Fprintf(output, FORMAT, "Client", "Status", "Number", "Note")
	for _, resourceDownload := range resourceDownloads {
		var notes []string
		notes = append(notes, "id:"+resourceDownload.ResourceId)
		if resourceDownload.SavePath != "" {
			notes = append(notes, resourceDownload.SavePath)
		}
		notes = append(notes, "tags:"+strings.Join(resourceDownload.Tags, ","))
		stringutil.PrintStringInWidth(output, resourceDownload.Title, NAME_WIDTH, true)
		fmt.Fprintf(output, FORMAT, resourceDownload.Client, resourceDownload.Status,
			resourceDownload.Number, strings.Join(notes, " ; "))
	}
}

func Init(dbfile string, verboseLevel int) (db *gorm.DB, err error) {
	var logLevel logger.LogLevel = logger.Silent
	// switch verboseLevel {
	// case 0:
	// 	logLevel = logger.Silent
	// case 1:
	// 	logLevel = logger.Error
	// case 2:
	// 	logLevel = logger.Warn
	// case 3:
	// 	logLevel = logger.Info
	// }
	db, err = gorm.Open(sqlite.Open(dbfile), &gorm.Config{
		Logger: logger.Default.LogMode(logLevel),
	})
	if err != nil {
		return
	}
	err = db.AutoMigrate(&Download{}, &ResourceDownload{})
	return
}
