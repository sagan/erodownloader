// aria2 client: https://aria2.github.io/manual/en/html/aria2c.html .
// use it's json-rpc interface.
package aria2

import (
	"fmt"
	"net/url"
	"path/filepath"
	"strings"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/util"
)

type Aria2Client struct {
	name   string
	config *config.ClientConfig
}

// Delete implements client.Client.
func (a *Aria2Client) Delete(id string) (err error) {
	params := a.params(id)
	// remove && forceRemove only works on non-finished tasks. For finished tasks (completed/error/removed),
	// it throws 400 error. Use removeDownloadResult for those tasks instead.
	// The above applies to http / ftp tasks. BitTorrent tasks may differ.
	err = a.jsonRpc("aria2.forceRemove", params, nil)
	err2 := a.jsonRpc("aria2.removeDownloadResult", params, nil)
	if err != nil && strings.Contains(err.Error(), "status=400") {
		return err2
	}
	return err
}

// Pause implements client.Client.
func (a *Aria2Client) Pause(id string) error {
	var result string
	err := a.jsonRpc("aria2.forcePause", a.params(id), &result)
	if err == nil && result != "OK" {
		err = fmt.Errorf("result error: %q", result)
	}
	return err
}

// Resume implements client.Client.
func (a *Aria2Client) Resume(id string) error {
	return a.jsonRpc("aria2.unpause", a.params(id), nil)
}

// Generate a common params for aria2 rpc method.
func (a *Aria2Client) params(args ...any) []any {
	p := []any{}
	if a.config.Token != "" {
		p = append(p, "token:"+a.config.Token)
	}
	p = append(p, args...)
	return p
}

func (a *Aria2Client) GetAll() (downloads client.Downloads, err error) {
	downloads = client.Downloads{}
	var dls []*ApiStatus
	if err = a.jsonRpc("aria2.tellActive", a.params(), &dls); err != nil {
		return
	}
	for _, dl := range dls {
		downloads[dl.Gid] = dl
	}
	dls = nil
	// params: offset, num
	if err = a.jsonRpc("aria2.tellWaiting", a.params(0, 9999), &dls); err != nil {
		return
	}
	for _, dl := range dls {
		downloads[dl.Gid] = dl
	}
	dls = nil
	if err = a.jsonRpc("aria2.tellStopped", a.params(0, 9999), &dls); err != nil {
		return
	}
	for _, dl := range dls {
		if dl.Aria2Status == "removed" {
			continue
		}
		downloads[dl.Gid] = dl
	}
	return
}

// GetStatus implements client.Client.
func (a *Aria2Client) GetStatus() (client.Status, error) {
	var status *ApiGlobalStat
	err := a.jsonRpc("aria2.getGlobalStat", a.params(), &status)
	return status, err
}

// Get implements client.Client.
func (a *Aria2Client) Get(id string) (download client.Download, err error) {
	var status *ApiStatus
	err = a.jsonRpc("aria2.tellStatus", a.params(id), &status)
	if err != nil {
		return nil, err
	}
	if status.Aria2Status == "removed" {
		status = nil
	}
	return status, nil
}

func (a *Aria2Client) ChangeUrl(id string, url string) (err error) {
	params := a.params(id) // [secret, gid]
	var uris *ApiUris
	if err = a.jsonRpc("aria2.getUris", params, &uris); err != nil {
		return fmt.Errorf("failed to get uris: %w", err)
	}
	var result []any
	params = append(params, 1) // fileIndex, 1-based
	var delUris = []string{}
	for _, uri := range *uris {
		delUris = append(delUris, uri.Uri)
	}
	params = append(params, delUris, []string{url}) // [secret, gid, fileIndex, delUris, addUris]
	return a.jsonRpc("aria2.changeUri", params, &result)
}

// https://aria2.github.io/manual/en/html/aria2c.html#aria2.addUri
func (a *Aria2Client) Add(download client.DownloadTask) (id string, err error) {
	savePath := download.GetSavePath()
	if savePath == "" {
		savePath = a.config.SavePath
	}
	if a.config.Local && download.GetFilename() != "" {
		localpath := filepath.Join(savePath, download.GetFilename())
		if util.FileExists(localpath) && !util.FileExists(localpath+".aria2") {
			return "", client.ErrFileExists
		}
	}
	params := a.params()
	downloadUrl := download.GetUrl()
	downloadUrlObj, err := url.Parse(downloadUrl)
	if err != nil {
		return "", fmt.Errorf("invalid download url: %w", err)
	}
	params = append(params, []string{downloadUrl})
	var header []string
	header = append(header, "Host: "+downloadUrlObj.Host)
	if cookieStr := config.Data.GetCookieHeader(downloadUrlObj); cookieStr != "" {
		header = append(header, "Cookie: "+cookieStr)
	}
	params = append(params, &ApiInputOptions{
		Dir:       savePath,
		Out:       download.GetFilename(),
		Pause:     fmt.Sprint(download.GetPaused()),
		UserAgent: config.Data.UserAgent,
		Header:    header,
	})
	err = a.jsonRpc("aria2.addUri", params, &id)
	return
}

func (a *Aria2Client) GetConfig() *config.ClientConfig {
	return a.config
}

func Creator(name string, sc *config.ClientConfig, c *config.Config) (client.Client, error) {
	return &Aria2Client{
		name:   sc.Name,
		config: sc,
	}, nil
}

func init() {
	client.Register(&client.RegInfo{
		Name:    "aria2",
		Creator: Creator,
	})
}

var _ client.Client = (*Aria2Client)(nil)
