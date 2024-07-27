package web

import (
	"embed"
	"encoding/json"
	"fmt"
	"io/fs"
	"net/http"
	"net/url"
	"os"
	"strings"

	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/site"
	"github.com/sagan/erodownloader/util"
	log "github.com/sirupsen/logrus"
)

type ApiFunc func(params url.Values) (data any, err error)

type FileAddPayload struct {
	Id   string
	Name string
}

type ResourceAddPayload struct {
	Id     string
	Title  string
	Number string
	Author string
	Size   int64
	Tags   []string
}

//go:embed dist
var Webfs embed.FS

var ApiFuncs = map[string]ApiFunc{
	"basic":              Basic,
	"search":             Search,
	"searchr":            Searchr,
	"add":                Add,
	"addr":               Addr,
	"delete":             Delete,
	"deleter":            Deleter,
	"restart":            Restart,
	"restartr":           Restartr,
	"downloads":          Downloads,
	"resource_downloads": ResourceDownloads,
}

var apiHandler = http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
	if r.Method != http.MethodGet && r.Method != http.MethodOptions && r.Method != http.MethodPost {
		w.WriteHeader(400)
		return
	}
	r.ParseForm()
	w.Header().Add("Content-Type", "application/json")
	cors(w) // for now

	funcName := ""
	if strings.HasPrefix(r.URL.Path, "/api/") {
		funcName = r.URL.Path[5:]
	} else {
		funcName = r.Form.Get("func")
	}
	if config.Data.Token != "" && r.Form.Get("token") != config.Data.Token {
		w.WriteHeader(403)
	} else if funcName == "" {
		w.WriteHeader(404)
	} else if apiFunc := ApiFuncs[funcName]; apiFunc == nil {
		w.WriteHeader(404)
	} else if data, err := apiFunc(r.Form); err != nil {
		w.WriteHeader(500)
		util.PrintJson(w, err.Error())
	} else {
		util.PrintJson(w, data)
	}
})

func Start() error {
	mux := http.NewServeMux()
	root, _ := fs.Sub(Webfs, "dist")
	fileServer := http.FileServer(http.FS(root))
	indexHtml, _ := Webfs.ReadFile("dist/index.html")
	mux.Handle("/api/", apiHandler)
	mux.Handle("/api", apiHandler)
	mux.Handle("/", http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		path := strings.TrimPrefix(r.URL.Path, "/")
		f, err := root.Open(path)
		if err == nil {
			defer f.Close()
		}
		if path == "" || os.IsNotExist(err) {
			w.Write(indexHtml)
			return
		}
		fileServer.ServeHTTP(w, r)
	}))
	log.Warnf("Start http server at %d port", config.Data.Port)
	if config.Data.Token != "" {
		log.Warnf(`Token is enabled, use http://0.0.0.0:%d/?token=%s to access`, config.Data.Port, config.Data.Token)
	}
	return http.ListenAndServe(fmt.Sprintf(":%d", config.Data.Port), mux)
}

func Search(params url.Values) (any, error) {
	siteInstance, err := site.CreateSite(params.Get("site"))
	if err != nil {
		return nil, fmt.Errorf("failed to create site: %w", err)
	}
	log.Printf("Search qs: %q", params.Get("qs"))
	return siteInstance.Search(params.Get("qs"))
}

func Searchr(params url.Values) (any, error) {
	siteInstance, err := site.CreateSite(params.Get("site"))
	if err != nil {
		return nil, fmt.Errorf("failed to create site: %w", err)
	}
	log.Printf("Searchr qs: %q", params.Get("qs"))
	return siteInstance.SearchResources(params.Get("qs"))
}

func Add(params url.Values) (data any, err error) {
	var files []*FileAddPayload
	err = json.Unmarshal([]byte(params.Get("files")), &files)
	if err != nil {
		return nil, fmt.Errorf("invalid files: %w", err)
	}
	var successFileIds []string
	var errs []error
	for _, file := range files {
		sitename := ""
		if values, err := url.ParseQuery(file.Id); err == nil {
			sitename = values.Get("site")
		}
		if sitename == "" {
			errs = append(errs, fmt.Errorf("invalid id: no sitename"))
			continue
		}
		siteInstance, err := site.CreateSite(sitename)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create site: %w", err))
			continue
		}
		identifier := siteInstance.GetIdentifier(file.Id)
		existingFile := schema.Download{}
		if result := config.Db.First(&existingFile, "site = ? and identifier = ?",
			sitename, identifier); result.Error == nil {
			errs = append(errs, fmt.Errorf("file %s (%s) already downloaded before. skip it",
				existingFile.Filename, identifier))
			continue
		}
		download := &schema.Download{
			FileId:     file.Id,
			Filename:   file.Name,
			Identifier: identifier,
			Site:       sitename,
		}
		if result := config.Db.Create(download); result.Error != nil {
			errs = append(errs, fmt.Errorf("failed to add file %q to client: %v", file.Name, result.Error))
			continue
		}
		successFileIds = append(successFileIds, file.Id)
	}
	return map[string]any{
		"success": successFileIds,
		"errors":  errs,
	}, nil
}

func Addr(params url.Values) (data any, err error) {
	var resources []*ResourceAddPayload
	err = json.Unmarshal([]byte(params.Get("resources")), &resources)
	if err != nil {
		return nil, fmt.Errorf("invalid resources: %w", err)
	}
	var successResourceIds []string
	var errs []error
	for _, resource := range resources {
		sitename := ""
		if values, err := url.ParseQuery(resource.Id); err == nil {
			sitename = values.Get("site")
		}
		if sitename == "" {
			errs = append(errs, fmt.Errorf("invalid id: no sitename"))
			continue
		}
		siteInstance, err := site.CreateSite(sitename)
		if err != nil {
			errs = append(errs, fmt.Errorf("failed to create site: %w", err))
			continue
		}
		identifier := siteInstance.GetIdentifier(resource.Id)
		existingResource := schema.ResourceDownload{}
		if result := config.Db.First(&existingResource, "site = ? and identifier = ?",
			sitename, identifier); result.Error == nil {
			errs = append(errs, fmt.Errorf("resource %s (%s) already downloaded before. skip it",
				existingResource.Title, identifier))
			continue
		}
		resouceDownload := &schema.ResourceDownload{
			ResourceId: resource.Id,
			Identifier: identifier,
			Site:       sitename,
			Number:     resource.Number,
			Author:     resource.Author,
			Title:      resource.Title,
			Size:       resource.Size,
			Tags:       resource.Tags,
		}
		if result := config.Db.Create(resouceDownload); result.Error != nil {
			errs = append(errs, fmt.Errorf("failed to add resource %q to client: %v", resource.Title, result.Error))
			continue
		}
		successResourceIds = append(successResourceIds, resource.Id)
	}
	return map[string]any{
		"success": successResourceIds,
		"errors":  errs,
	}, nil
}

func Restart(params url.Values) (data any, err error) {
	ids := params["id"]
	result := config.Db.Model(&schema.Download{}).Where("id in ?", ids).Updates(map[string]any{
		"status":      "",
		"note":        "",
		"download_id": "",
	})
	return nil, result.Error
}

func Restartr(params url.Values) (data any, err error) {
	ids := params["id"]
	result := config.Db.Model(&schema.ResourceDownload{}).Where("id in ?", ids).Updates(map[string]any{
		"status": "",
		"note":   "",
	})
	return nil, result.Error
}

func Delete(params url.Values) (data any, err error) {
	ids := params["id"]
	result := config.Db.Where("id in ?", ids).Delete(&schema.Download{})
	return nil, result.Error
}

func Deleter(params url.Values) (data any, err error) {
	ids := params["id"]
	result := config.Db.Where("id in ?", ids).Delete(&schema.ResourceDownload{})
	return nil, result.Error
}

func Downloads(params url.Values) (data any, err error) {
	var downloads []schema.Download
	result := config.Db.Find(&downloads)
	return downloads, result.Error
}

func ResourceDownloads(params url.Values) (data any, err error) {
	var resourceDownloads []schema.ResourceDownload
	result := config.Db.Find(&resourceDownloads)
	return resourceDownloads, result.Error
}

func Basic(params url.Values) (data any, err error) {
	var sites []string
	for _, site := range config.InternalSites {
		sites = append(sites, site.Name)
	}
	for _, site := range config.Data.Sites {
		sites = append(sites, site.Name)
	}
	var clients []string
	for _, client := range config.InternalClients {
		clients = append(clients, client.Name)
	}
	for _, client := range config.Data.Clients {
		clients = append(clients, client.Name)
	}
	return map[string]any{
		"clients": clients,
		"sites":   sites,
	}, nil
}

func cors(w http.ResponseWriter) {
	w.Header().Add("Access-Control-Allow-Origin", "*")
	w.Header().Add("Access-Control-Allow-Credentials", "true")
	w.Header().Add("Access-Control-Allow-Headers", "Content-Type, Content-Length, Accept-Encoding, X-CSRF-Token, Authorization, accept, origin, Cache-Control, X-Requested-With")
	w.Header().Add("Access-Control-Allow-Methods", "POST, GET, OPTIONS, PUT, DELETE")
}
