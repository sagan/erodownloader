package aria2

import (
	"encoding/base64"
	"encoding/json"
	"fmt"
	"net/url"
	"path"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/httpclient"
	"github.com/sagan/erodownloader/util"
)

// Notable, all fields of aria2 rpc request & response is string type.
// number and boolean values use the literal string format.

type ApiResponse struct {
	Id      string // request id
	Jsonrpc string // "2.0"
	Result  json.RawMessage
}

// https://aria2.github.io/manual/en/html/aria2c.html#aria2.addUri
type ApiInputOptions struct {
	Dir                    string   `json:"dir,omitempty"`
	MaxConnectionPerServer string   `json:"max-connection-per-server,omitempty"` // default: 1
	Pause                  string   `json:"pause,omitempty"`
	Out                    string   `json:"out,omitempty"` // the download filename, related to dir
	Header                 []string `json:"header,omitempty"`
	UserAgent              string   `json:"user-agent,omitempty"`
}

// https://aria2.github.io/manual/en/html/aria2c.html#aria2.getGlobalStat
type ApiGlobalStat struct {
	Aria2DownloadSpeed string `json:"downloadSpeed,omitempty"`
	Aria2UploadSpeed   string `json:"uploadSpeed,omitempty"`
}

type ApiFile struct {
	Index  string `json:"index,omitempty"`
	Path   string `json:"path,omitempty"`
	Length string `json:"length,omitempty"`
}

// https://aria2.github.io/manual/en/html/aria2c.html#aria2.tellStatus
type ApiStatus struct {
	Gid             string     `json:"gid,omitempty"`
	Aria2Status     string     `json:"status,omitempty"` // active|waiting|paused|error|complete|removed
	Dir             string     `json:"dir,omitempty"`
	TotalLength     string     `json:"totalLength,omitempty"`
	CompletedLength string     `json:"completedLength,omitempty"`
	Files           []*ApiFile `json:"files,omitempty"`
	ErrorCode       string     `json:"errorCode,omitempty"`
	ErrorMessage    string     `json:"errorMessage,omitempty"`
}

// Size implements client.Download.
func (a *ApiStatus) Size() int64 {
	if len(a.Files) != 1 {
		return 0
	}
	return util.First(util.RAMInBytes(a.Files[0].Length))
}

func (a *ApiStatus) Msg() string {
	if a.ErrorCode != "" && a.ErrorCode != "0" {
		return fmt.Sprintf("Err-%s:%s", a.ErrorCode, a.ErrorMessage)
	}
	return ""
}

// Filename implements client.Download.
func (a *ApiStatus) Filename() string {
	if len(a.Files) != 1 {
		return ""
	}
	if a.Files[0].Path != "" {
		return path.Base(a.Files[0].Path)
	}
	return ""
}

// SavePath implements client.Download.
func (a *ApiStatus) SavePath() string {
	return a.Dir
}

// https://aria2.github.io/manual/en/html/aria2c.html#aria2.getUris
type ApiUris []*struct {
	// 'used' if the URI is in use. 'waiting' if the URI is still waiting in the queue.
	Status string `json:"status,omitempty"`
	Uri    string `json:"uri,omitempty"`
}

func (a *ApiStatus) Id() string {
	return a.Gid
}

func (a *ApiStatus) Status() string {
	switch a.Aria2Status {
	case "active", "waiting":
		return "downloading"
	case "complete":
		return "completed"
	case "removed":
		return "deleted"
	case "paused", "error":
		return a.Aria2Status
	}
	return "unknown"
}

func (ags *ApiGlobalStat) DownloadSpeed() int64 {
	return int64(util.First(util.ParseInt(ags.Aria2DownloadSpeed, 0)))
}

func (ags *ApiGlobalStat) UploadSpeed() int64 {
	return int64(util.First(util.ParseInt(ags.Aria2UploadSpeed, 0)))
}

func (a *Aria2Client) jsonRpc(method string, params []any, v any) (err error) {
	apiUrl := a.config.Url
	if apiUrl == "" {
		return fmt.Errorf("no url")
	}
	urlObj, err := url.Parse(apiUrl)
	if err != nil {
		return fmt.Errorf("incorrect url: %w", err)
	}
	query := urlObj.Query()
	query.Set("method", method)
	query.Set("id", "a")
	paramsJson, err := json.Marshal(params)
	if err != nil {
		return fmt.Errorf("invalid params: %w", err)
	}
	query.Set("params", base64.StdEncoding.EncodeToString(paramsJson))
	urlObj.RawQuery = query.Encode()
	var apiResponse *ApiResponse
	err = httpclient.FetchJson(urlObj.String(), &apiResponse, false)
	if err != nil || apiResponse == nil || apiResponse.Result == nil {
		return fmt.Errorf("invalid response: %w", err)
	}
	if v != nil {
		err = json.Unmarshal(apiResponse.Result, v)
	}
	return
}

var _ client.Status = (*ApiGlobalStat)(nil)
var _ client.Download = (*ApiStatus)(nil)
