package client

import (
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/stringutil"
)

// "Id", "Status", "Size", msgWidth, "Msg", "Path"
const DOWNLOAD_LIST_FORMAT = "%-16s  %-12s  %-7s  %-*s  %-s\n"
const DOWNLOAD_LIST_DEFAULT_MSG_WIDTH = 40

type DownloadTask interface {
	GetUrl() string
	GetFilename() string // "foobar.rar"
	GetSavePath() string // "/root/Downloads"
	GetPaused() bool     // add task in paused state
}

type Download interface {
	Id() string // download task id in client
	Filename() string
	Size() int64
	SavePath() string
	Status() string // downloading|paused|completed|error|deleted|unknown
	Msg() string
}

type Status interface {
	DownloadSpeed() int64
}

// id => Download
type Downloads map[string]Download

// Modeled after aria2 APIs.
// Notable, there is no way to resume an error task, the only choice is to re-create a new one.
// Currently, only http / ftp tasks is supported (a task has only one (1) file)
type Client interface {
	GetStatus() (Status, error)
	GetConfig() *config.ClientConfig
	Add(DownloadTask) (id string, err error) // Return created download id
	Get(id string) (download Download, err error)
	Delete(id string) error
	Pause(id string) error
	Resume(id string) error // Note it will throw an error if task state is "error"
	GetAll() (Downloads, error)
	ChangeUrl(id string, url string) error
}

type RegInfo struct {
	Name    string
	Creator func(string, *config.ClientConfig, *config.Config) (Client, error)
}

type BaseDownloadTask struct {
	Url      string
	Filename string
	SavePath string
	Paused   bool
}

var (
	ErrFileExists = fmt.Errorf("local file exists")
)

func (b *BaseDownloadTask) GetFilename() string {
	return b.Filename
}

func (b *BaseDownloadTask) GetPaused() bool {
	return b.Paused
}

func (b *BaseDownloadTask) GetSavePath() string {
	return b.SavePath
}

func (b *BaseDownloadTask) GetUrl() string {
	return b.Url
}

var (
	registryMap = map[string]*RegInfo{}
	clients     = map[string]Client{}
	mu          sync.Mutex
)

func (d Downloads) Print(output io.Writer, filter string) {
	msgWidth := 10
	var downloading, completed, others []string
	for id := range d {
		msgWidth = max(msgWidth, len(d[id].Msg()))
		switch d[id].Status() {
		case "downloading":
			downloading = append(downloading, id)
		case "completed":
			completed = append(completed, id)
		default:
			others = append(others, id)
		}
	}
	msgWidth = min(msgWidth, 60)
	PrintDownloadListHeader(output, msgWidth)
	var queues = [][]string{downloading, completed, others}
	for _, queue := range queues {
		for _, id := range queue {
			if filter != "" && !MatchDownloadWitchFilter(d[id], filter) {
				continue
			}
			PrintDownload(output, d[id], msgWidth)
		}
	}
}

func Sep(client Client) string {
	if client.GetConfig().Windows {
		return `\`
	}
	return `/`
}

func MatchDownloadWitchFilter(download Download, filter string) bool {
	return stringutil.ContainsI(download.Filename(), filter) || stringutil.ContainsI(download.SavePath(), filter)
}

func PrintDownloadListHeader(output io.Writer, msgWidth int) {
	if msgWidth <= 0 {
		msgWidth = DOWNLOAD_LIST_DEFAULT_MSG_WIDTH
	}
	fmt.Fprintf(output, DOWNLOAD_LIST_FORMAT, "Id", "Status", "Size", msgWidth, "Msg", "Path")
}

func PrintDownload(output io.Writer, d Download, msgWidth int) {
	if msgWidth <= 0 {
		msgWidth = DOWNLOAD_LIST_DEFAULT_MSG_WIDTH
	}
	msg := d.Msg()
	if len(msg) > msgWidth {
		msg = msg[:msgWidth]
	}
	fullpath := d.SavePath()
	if strings.Contains(fullpath, `\`) {
		fullpath += `\`
	} else {
		fullpath += "/"
	}
	if d.Filename() != "" {
		fullpath += d.Filename()
	}
	fmt.Fprintf(output, DOWNLOAD_LIST_FORMAT, d.Id(), d.Status(),
		util.BytesSize(float64(d.Size())), msgWidth, msg, fullpath)
}

func Register(regInfo *RegInfo) {
	registryMap[regInfo.Name] = regInfo
}

func CreateClient(name string) (Client, error) {
	mu.Lock()
	defer mu.Unlock()
	if clients[name] != nil {
		return clients[name], nil
	}
	clientConfig := config.GetClientConfig(name)
	if clientConfig == nil {
		return nil, fmt.Errorf("client %s not found", name)
	}
	clientInstance, err := CreateClientInternal(name, clientConfig, config.Data)
	if err != nil {
		clients[name] = clientInstance
	}
	return clientInstance, err
}

func CreateClientInternal(name string, clientConfig *config.ClientConfig, config *config.Config) (Client, error) {
	regInfo := registryMap[clientConfig.Type]
	if regInfo == nil {
		return nil, fmt.Errorf("unsupported client type %s", name)
	}
	return regInfo.Creator(name, clientConfig, config)
}

func PrintStatus(output io.Writer, s Status) {
	fmt.Fprintf(output, "DownloadSpeed: %s\n", util.BytesSize(float64(s.DownloadSpeed())))
}

var _ DownloadTask = (*BaseDownloadTask)(nil)
var _ DownloadTask = (*schema.Download)(nil)
