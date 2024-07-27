package alist

import (
	"encoding/json"
	"net/url"
	"path"
	"time"

	"github.com/sagan/erodownloader/site"
)

// api/fs/get: password & path. api/fs/list: all fields.
type ApiRequest struct {
	Page     int    `json:"page,omitempty"`
	Password string `json:"password,omitempty"`
	Path     string `json:"path,omitempty"` // "/Directory"
	PerPage  int    `json:"per_page,omitempty"`
	Refresh  bool   `json:"refresh,omitempty"`
	Parent   string `json:"parent,omitempty"`   // for fs/search api. "/".
	Keywords string `json:"keywords,omitempty"` // for fs/search api
	Scope    int    `json:"scope,omitempty"`    // fs/search: 0
}

type ApiResponse struct {
	Code    int             `json:"code,omitempty"`
	Message string          `json:"message,omitempty"`
	Data    json.RawMessage `json:"data,omitempty"`
}

type ApiFile struct {
	Modified   string `json:"modified,omitempty"` // "2024-05-08T04:04:18.031Z" or "2024-06-26T21:17:23.200693154+08:00"
	ItemName   string `json:"name,omitempty"`
	Provider   string `json:"provider,omitempty"`
	ItemRawUrl string `json:"raw_url,omitempty"`
	Sign       string `json:"sign,omitempty"`
	ItemSize   int64  `json:"size,omitempty"`
	ItemIsDir  bool   `json:"is_dir,omitempty"`
	Hashinfo   string `json:"hashinfo,omitempty"` // could be empty or "null"
	Type       int    `json:"type,omitempty"`
	Parent     string `json:"parent,omitempty"` // "/Foo/Bar". Only has value in search api.
	password   string
	site       string
	full       bool
}

// Tags implements site.File.
func (f *ApiFile) Tags() []string {
	return nil
}

// IsFull implements site.File.
func (f *ApiFile) IsFull() bool {
	return f.full
}

// Site implements site.File.
func (f *ApiFile) Site() string {
	return f.site
}

func (f *ApiFile) Id() string {
	if f.Parent == "" || f.ItemName == "" {
		return ""
	}
	values := url.Values{}
	values.Set("path", path.Join(f.Parent, f.ItemName))
	if f.password != "" {
		values.Set("password", f.password)
	}
	if f.site != "" {
		values.Set("site", f.site)
	}
	return values.Encode()
}

func (f *ApiFile) IsDir() bool {
	return f.ItemIsDir
}

func (f *ApiFile) RawUrl() string {
	return f.ItemRawUrl
}

func (f *ApiFile) Size() int64 {
	return f.ItemSize
}

var timeFormats = []string{"2006-01-02T15:04:05Z", "2006-01-02T15:04:05-07:00"}

func (f *ApiFile) Time() int64 {
	for _, format := range timeFormats {
		t, err := time.Parse(format, f.Modified)
		if err == nil {
			return t.Unix()
		}
	}
	return time.Unix(0, 0).Unix()
}

type ApiList struct {
	Total    int
	Readme   string
	Provider string
	Content  []*ApiFile
}

func (f *ApiFile) Name() string {
	return f.ItemName
}

var _ site.File = (*ApiFile)(nil)
