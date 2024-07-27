package alist

import (
	"fmt"
	"net/url"
	"path"
	"strings"

	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/httpclient"
	"github.com/sagan/erodownloader/site"
	"github.com/sagan/erodownloader/util"
)

type AlistSite struct {
	Name             string
	Config           *config.SiteConfig
	ResourceProvider site.ResourceProvider
	rootName         string
}

func (a *AlistSite) GetIdentifier(id string) (identifier string) {
	values, err := url.ParseQuery(id)
	if err != nil {
		return ""
	}
	if values.Get("type") == "resource" {
		return values.Get("number")
	}
	return values.Get("path")
}

func (a *AlistSite) GetResourceFiles(id string) (site.Files, error) {
	if a.ResourceProvider != nil {
		return a.ResourceProvider.FindResourceFiles(id)
	}
	return nil, fmt.Errorf("resources not supported in this site")
}

func (a *AlistSite) ReadDir(id string) (site.Files, error) {
	if id == "" {
		return nil, fmt.Errorf("no id")
	}
	values, err := url.ParseQuery(id)
	if err != nil {
		return nil, fmt.Errorf("malformed id: %w", err)
	}
	if values.Get("path") == "" {
		return nil, fmt.Errorf("empty path")
	}
	return a.fsList(values)
}

func (a *AlistSite) SearchResources(qs string) (site.Resources, error) {
	if a.ResourceProvider != nil {
		return a.ResourceProvider.FindResources(qs)
	}
	return nil, fmt.Errorf("resources not supported in this site")
}

func (a *AlistSite) GetFile(id string) (site.File, error) {
	if id == "" {
		return nil, fmt.Errorf("no id")
	}
	values, err := url.ParseQuery(id)
	if err != nil {
		return nil, fmt.Errorf("malformed id: %w", err)
	}
	if values.Get("path") == "" {
		return nil, fmt.Errorf("empty path")
	}
	return a.fsGet(values.Get("path"), values.Get("password"))
}

func (a *AlistSite) fsGet(filepath string, password string) (site.File, error) {
	urlStr := a.Config.Url + "api/fs/get"
	req := &ApiRequest{Path: filepath, Password: password}
	var res *ApiResponse
	err := httpclient.PostAndFetchJson(urlStr, req, &res, true)
	if err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("api %d error, msg=%s", res.Code, res.Message)
	}
	file, err := util.UnmarshalJson[*ApiFile](res.Data)
	if file != nil {
		if file.Parent == "" {
			file.Parent = path.Dir(filepath)
		}
		file.full = true
		file.site = a.Name
		file.ItemName = util.CleanBasename(file.ItemName)
	}
	return file, err
}

func (a *AlistSite) fsList(params url.Values) (files site.Files, err error) {
	if a.rootName == "" {
		file, err := a.fsGet("/", "")
		if err != nil {
			return nil, fmt.Errorf("failed to get root file: %w", err)
		}
		a.rootName = file.Name()
	}
	req := &ApiRequest{
		Keywords: params.Get("q"),
		Path:     params.Get("path"),
		Password: params.Get("password"),
		Parent:   params.Get("parent"),
		Scope:    util.ParseInt(params.Get("scope"), 0),
		Page:     util.ParseInt(params.Get("page"), 1),
		PerPage:  util.ParseInt(params.Get("per_page"), 100),
	}
	apiUrl := a.Config.Url
	if req.Keywords != "" {
		apiUrl += "api/fs/search"
	} else if req.Path != "" {
		apiUrl += "api/fs/list"
	} else {
		return nil, fmt.Errorf("invalid params")
	}
	var res *ApiResponse
	err = httpclient.PostAndFetchJson(apiUrl, req, &res, true)
	if err != nil {
		return nil, err
	}
	if res.Code != 200 {
		return nil, fmt.Errorf("api %d error, msg=%s", res.Code, res.Message)
	}
	content, err := util.UnmarshalJson[*ApiList](res.Data)
	if err != nil {
		return nil, err
	}
	for _, file := range content.Content {
		if file.Parent == "" && req.Path != "" {
			file.Parent = req.Path
		}
		file.ItemName = util.CleanBasename(file.ItemName)
		// 有时 alist search API 返回的 file 的 path 里包含 /root_name 前缀。
		file.Parent = strings.TrimPrefix(file.Parent, "/"+a.rootName)
		file.site = a.Name
		files = append(files, file)
	}
	return files, nil
}

func (a *AlistSite) GetConfig() *config.SiteConfig {
	return a.Config
}

func (a *AlistSite) Search(qs string) (site.Files, error) {
	query, err := url.ParseQuery(qs)
	if err != nil {
		return nil, err
	}
	if query.Get("id") != "" {
		if file, err := a.GetFile(query.Get("id")); err != nil {
			return nil, err
		} else {
			return site.Files{file}, nil
		}
	}
	return a.fsList(query)
}

func New(name string, sc *config.SiteConfig, resourceProvider site.ResourceProvider) (*AlistSite, error) {
	if sc.Url == "" {
		return nil, fmt.Errorf("site url can not be empty")
	}
	return &AlistSite{
		Name:             sc.Name,
		Config:           sc,
		ResourceProvider: resourceProvider,
	}, nil
}

func Creator(name string, sc *config.SiteConfig, c *config.Config) (site.Site, error) {
	return New(sc.Name, sc, nil)
}

func init() {
	site.Register(&site.RegInfo{
		Name:    "alist",
		Creator: Creator,
	})
}

var _ site.Site = (*AlistSite)(nil)
