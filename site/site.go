package site

import (
	"encoding/json"
	"fmt"
	"io"
	"strings"
	"sync"

	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/stringutil"
)

// All funcs of File, all methods should not have any side effect (e.g. access network).
type File interface {
	Site() string
	Id() string
	Name() string // canonical filename
	Size() int64
	IsDir() bool
	Time() int64    // modified unix timestampp (seconds)
	RawUrl() string // Get file raw download url
	IsFull() bool   // false: file may be incomplete. use site.Get(file.Id()) to get full file object.
	Tags() []string
}

type Files []File

// A resource represent a collection of files
type Resource interface {
	Site() string
	Id() string
	Number() string // メイン番号, e.g. RJ123456
	Title() string
	Author() string // 作者 / 社团 / 公司
	Size() int64    // not guaranteed to be accurate
	Tags() schema.Tags
}

type Resources []Resource

type ResourceProvider interface {
	FindResources(id string) (Resources, error)
	FindResourceFiles(qs string) (Files, error)
}

type Site interface {
	// qs can be any site specific string. Query string format is recommended, e.g. "q=foo&category=bar"
	Search(qs string) (Files, error)
	// Search resources. qs is query string. if qs is empty, return all resources
	SearchResources(qs string) (Resources, error)
	GetConfig() *config.SiteConfig // Get effective config
	GetFile(id string) (File, error)
	ReadDir(id string) (Files, error) // pass the id of a dir, get it's contents
	GetResourceFiles(id string) (Files, error)
	// Return the permanent (non-changing) part of the id, should be the same result for the same object.
	// It should not have any side effect.
	// id vs identifier: id may contains additional information that's required to access the object
	// (e.g. password or authorization token) in addition to identifier; id may become stale / invalid
	// (e.g. site changes the password), but the identifier always remains the same.
	GetIdentifier(id string) (identifier string)
}

type RegInfo struct {
	Name    string
	Creator func(string, *config.SiteConfig, *config.Config) (Site, error)
}

var (
	registryMap = map[string]*RegInfo{}
	sites       = map[string]Site{}
	mu          sync.Mutex
)

func Register(regInfo *RegInfo) {
	registryMap[regInfo.Name] = regInfo
}

func CreateSite(name string) (Site, error) {
	mu.Lock()
	defer mu.Unlock()
	if sites[name] != nil {
		return sites[name], nil
	}
	siteConfig := config.GetSiteConfig(name)
	if siteConfig == nil {
		return nil, fmt.Errorf("site %s not found", name)
	}
	siteInstance, err := CreateSiteInternal(name, siteConfig, config.Data)
	if err != nil {
		sites[name] = siteInstance
	}
	return siteInstance, err
}

func CreateSiteInternal(name string, siteConfig *config.SiteConfig, config *config.Config) (Site, error) {
	regInfo := registryMap[siteConfig.Type]
	if regInfo == nil {
		return nil, fmt.Errorf("unsupported site type %s", name)
	}
	return regInfo.Creator(name, siteConfig, config)
}

func (fs Files) Print(output io.Writer) {
	nameWidth := 40
	format := "%4s  %-19s  %-6s  %s\n"
	fmt.Fprintf(output, "%-*s"+format, nameWidth, "Name", "Flag", "Time", "Size", "Details")
	for _, f := range fs {
		flag := "-"
		if f.IsDir() {
			flag = "d"
		}
		var details []string
		if f.Id() != "" {
			details = append(details, "id: "+f.Id())
		}
		if f.RawUrl() != "" {
			details = append(details, "rawUrl: "+f.RawUrl())
		}
		stringutil.PrintStringInWidth(output, f.Name(), nameWidth, true)
		fmt.Fprintf(output, format, flag, util.FormatTime(f.Time()),
			util.BytesSizeAround(float64(f.Size())), strings.Join(details, " ; "))
	}
}

func (rs Resources) Size() (size int64) {
	for _, r := range rs {
		size += r.Size()
	}
	return size
}

func (rs Resources) Print(output io.Writer) {
	nameWidth := 80
	format := "%6s  %-15s  %s\n"
	fmt.Fprintf(output, "%-*s"+format, nameWidth, "Name", "Size", "Number", "Details")
	for _, r := range rs {
		var details []string
		if tags := r.Tags(); len(tags) > 0 {
			details = append(details, strings.Join(tags, ","))
		}
		if id := r.Id(); id != "" {
			details = append(details, " // "+id)
		}
		var name string
		if r.Author() != "" {
			name = fmt.Sprintf("[%s]%s", r.Author(), r.Title())
		} else {
			name = r.Title()
		}
		stringutil.PrintStringInWidth(output, name, nameWidth, true)
		fmt.Fprintf(output, format, util.BytesSizeAround(float64(r.Size())), r.Number(), strings.Join(details, " ; "))
	}
}

func (rs Resources) MarshalJSON() ([]byte, error) {
	var resources []map[string]any
	for _, r := range rs {
		resources = append(resources, map[string]any{
			"id":     r.Id(),
			"title":  r.Title(),
			"number": r.Number(),
			"author": r.Author(),
			"size":   r.Size(),
			"tags":   r.Tags(),
		})
	}
	return json.Marshal(resources)
}

func (fs Files) MarshalJSON() ([]byte, error) {
	var files []map[string]any
	for _, f := range fs {
		files = append(files, map[string]any{
			"id":      f.Id(),
			"is_dir":  f.IsDir(),
			"time":    f.Time(),
			"name":    f.Name(),
			"size":    f.Size(),
			"tags":    f.Tags(),
			"raw_url": f.RawUrl(),
		})
	}
	return json.Marshal(files)
}
