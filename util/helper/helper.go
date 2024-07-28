package helper

import (
	"errors"
	"fmt"
	"io"
	"net/url"
	"os"
	"os/exec"
	"path/filepath"
	"regexp"
	"strings"

	"github.com/natefinch/atomic"
	log "github.com/sirupsen/logrus"
	"golang.org/x/term"

	"github.com/sagan/erodownloader/client"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/site"
	"github.com/sagan/erodownloader/util"
)

type PtoolSearchResult struct {
	ErrorSites    int `json:"errorSites,omitempty"`
	NoResultSites int `json:"noResultSites,omitempty"`
	SuccessSites  int `json:"successSites,omitempty"`
	Torrents      []*struct {
		Name        string `json:"Name,omitempty"`
		Description string `json:"Description,omitempty"`
		Id          string `json:"Id,omitempty"`
		Size        int64  `json:"Size,omitempty"`
	} `json:"torrents,omitempty"`
}

func ParseFilenameArgs(args ...string) []string {
	names := []string{}
	for _, arg := range args {
		filenames := GetWildcardFilenames(arg)
		if filenames == nil {
			names = append(names, arg)
		} else {
			names = append(names, filenames...)
		}
	}
	return names
}

// "*.torrent" => ["./a.torrent", "./b.torrent"...].
// Return nil if filestr does not contains wildcard char.
// Windows cmd / powershell 均不支持命令行 *.torrent 参数扩展。必须应用自己实现。做个简易版的.
func GetWildcardFilenames(filestr string) []string {
	if !strings.ContainsAny(filestr, "*") {
		return nil
	}
	dir := filepath.Dir(filestr)
	name := filepath.Base(filestr)
	ext := filepath.Ext(name)
	if ext != "" {
		name = name[:len(name)-len(ext)]
	}
	prefix := ""
	suffix := ""
	exact := ""
	index := strings.Index(name, "*")
	if index != -1 {
		prefix = name[:index]
		suffix = name[index+1:]
	} else {
		exact = name
	}
	entries, err := os.ReadDir(dir)
	if err != nil {
		return nil
	}
	filenames := []string{}
	for _, entry := range entries {
		entryName := entry.Name()
		entryExt := filepath.Ext(entryName)
		if ext != "" {
			if entryExt == "" || (entryExt != ext && ext != ".*") {
				continue
			}
			entryName = entryName[:len(entryName)-len(entryExt)]
		}
		if exact != "" && entryName != exact {
			continue
		}
		if prefix != "" && !strings.HasPrefix(entryName, prefix) {
			continue
		}
		if suffix != "" && !strings.HasSuffix(entryName, suffix) {
			continue
		}
		filenames = append(filenames, dir+string(filepath.Separator)+entry.Name())
	}
	return filenames
}

// Ask user to confirm an (dangerous) action via typing yes in tty
func AskYesNoConfirm(prompt string) bool {
	if prompt == "" {
		prompt = "Will do the action"
	}
	fmt.Fprintf(os.Stderr, "%s, are you sure? (yes/no): ", prompt)
	if !term.IsTerminal(int(os.Stdin.Fd())) {
		fmt.Fprintf(os.Stderr, `Abort due to stdin is NOT tty. Use a proper flag (like "--force") to skip the prompt`+"\n")
		return false
	}
	for {
		input := ""
		fmt.Scanf("%s\n", &input)
		switch input {
		case "yes", "YES", "Yes":
			return true
		case "n", "N", "no", "NO", "No":
			return false
		default:
			if len(input) > 0 {
				fmt.Fprintf(os.Stderr, "Respond with yes or no (Or use Ctrl+C to abort): ")
			} else {
				return false
			}
		}
	}
}

// Add file or resource to client.
// If id is a resource id, add all files of it to client.
// It skip downloading files that already exists in local.
func AddDownloadTask(clientInstance client.Client, id string, savePath string) (
	clientDownloads []client.Download, downloads []*schema.Download, err error) {
	var values url.Values
	values, err = url.ParseQuery(id)
	if err != nil {
		err = fmt.Errorf("failed to parse id %q: %w", id, err)
		return
	}
	clientname := clientInstance.GetConfig().Name
	sitename := values.Get("site")
	if sitename == "" {
		err = fmt.Errorf("id %q has no site", id)
		return
	}
	siteInstance, err := site.CreateSite(sitename)
	if err != nil {
		err = fmt.Errorf("failed to create site %q: %w", sitename, err)
		return
	}
	if values.Get("type") == "resource" {
		var files site.Files
		files, err = siteInstance.GetResourceFiles(id)
		if err != nil {
			err = fmt.Errorf("failed to get resource %q files: %w", id, err)
			return
		}
		for _, file := range files {
			if file.RawUrl() == "" {
				if _file, _err := siteInstance.GetFile(file.Id()); _err != nil {
					err = fmt.Errorf("failed to get file %q full info: %w", file.Id(), _err)
					return
				} else {
					file = _file
				}
			}
			fileUrl := file.RawUrl()
			if fileUrl == "" {
				err = fmt.Errorf("file %q no url found", id)
				return
			}
			downloads = append(downloads, &schema.Download{
				SavePath:   savePath,
				Status:     "downloading",
				Client:     clientname,
				FileUrl:    fileUrl,
				FileId:     file.Id(),
				Site:       sitename,
				Identifier: siteInstance.GetIdentifier(file.Id()),
				Filename:   file.Name(),
				ResourceId: id,
			})
		}
	} else {
		var file site.File
		file, err = siteInstance.GetFile(id)
		if err != nil {
			err = fmt.Errorf("failed to get file %q full info: %w", id, err)
			return
		}
		fileUrl := file.RawUrl()
		if fileUrl == "" {
			err = fmt.Errorf("file %s no url", file.Name())
			return
		}
		downloads = append(downloads, &schema.Download{
			SavePath: savePath,
			Status:   "downloading",
			Client:   clientname,
			FileId:   file.Id(),
			FileUrl:  fileUrl,
			Filename: file.Name(),
		})
	}
	for _, download := range downloads {
		id, err = clientInstance.Add(download)
		if err != nil {
			if errors.Is(err, client.ErrFileExists) {
				log.Tracef("%s/%s file exists (already downloaded before)", download.GetSavePath(), download.GetFilename())
				err = nil
				download.Status = "completed"
				continue
			}
			err = fmt.Errorf("failed to add task: %w", err)
			return
		}
		clientDownload, _err := clientInstance.Get(id)
		if _err != nil {
			err = fmt.Errorf("failed to get added task: %w", _err)
			return
		}
		// client.PrintDownload(os.Stderr, download, 40)
		clientDownloads = append(clientDownloads, clientDownload)
		download.DownloadId = id
	}
	return
}

// Return fullpath = join(dir,name), suitable for creating a new file in dir.
// If file already exists, append the proper numeric suffix to make sure fullpath does not exist.
func GetNewFilePath(dir string, name string) (fullpath string) {
	if dir == "" || name == "" {
		return ""
	}
	fullpath = filepath.Join(dir, name)
	if !util.FileExists(fullpath) {
		return
	}
	i := 1
	ext := filepath.Ext(name)
	base := name[:len(name)-len(ext)]
	for {
		fullpath = filepath.Join(dir, fmt.Sprintf("%s.%d%s", base, i, ext))
		if !util.FileExists(fullpath) {
			return
		}
		i++
	}
}

func SearchPtoolSite(binary, site, keyword string, includeDead bool, matchExact bool) (exists bool, err error) {
	minSeeders := "1"
	if includeDead {
		minSeeders = "-1"
	}
	if binary == "" {
		binary = "ptool"
	}
	cmd := exec.Command(binary, "search", "--min-seeders", minSeeders, "--json", site, keyword)
	output, err := cmd.Output()
	if err != nil {
		return false, err
	}
	ptoolResult, err := util.UnmarshalJson[*PtoolSearchResult](output)
	if err != nil {
		return false, err
	}
	if ptoolResult.ErrorSites > 0 {
		return false, fmt.Errorf("search site fail")
	}
	if matchExact {
		regex := regexp.MustCompile(`\b` + regexp.QuoteMeta(keyword) + `(\b|_)`)
		for _, torrent := range ptoolResult.Torrents {
			if regex.MatchString(torrent.Name) || regex.MatchString(torrent.Description) {
				return true, nil
			}
		}
	} else if len(ptoolResult.Torrents) > 0 {
		return true, nil
	}
	return false, nil
}

func ReadFileHeader(name string, size int) ([]byte, error) {
	f, err := os.Open(name)
	if err != nil {
		return nil, err
	}
	defer f.Close()
	b := make([]byte, size)
	n, err := io.ReadAtLeast(f, b, size)
	if err == io.EOF || err == io.ErrUnexpectedEOF {
		err = nil
	}
	return b[:n], err
}

// Normalize file path names, truncate long names and replace restrictive chars.
func NormalizeName(continueOnError bool, pathes ...string) (renamed int, err error) {
	if len(pathes) == 0 {
		return 0, fmt.Errorf("no path provided")
	}
	errorCnt := 0
	renamedCnt := 0
	for len(pathes) > 0 {
		currentpath := pathes[0]
		pathes = pathes[1:]

		stat, err := os.Stat(currentpath)
		if err != nil {
			if !continueOnError {
				return renamed, err
			}
			log.Errorf("%q: %v", currentpath, err)
			errorCnt++
			continue
		}

		dir := filepath.Dir(currentpath)
		basename := filepath.Base(currentpath)

		var newbasename string
		if stat.IsDir() {
			newbasename = util.CleanBasename(basename)
		} else {
			newbasename = util.CleanFileBasename(basename)
		}
		if newbasename != basename {
			newpath := filepath.Join(dir, newbasename)
			if util.FileExists(newpath) {
				if !continueOnError {
					return renamed, err
				}
				log.Errorf("%q: rename target %q exists", currentpath, newbasename)
				errorCnt++
				continue
			}
			err = atomic.ReplaceFile(currentpath, newpath)
			if err != nil {
				if !continueOnError {
					return renamed, err
				}
				log.Errorf("%q => %q: %v", currentpath, newbasename, err)
				errorCnt++
				continue
			}
			log.Tracef("%q => %q\n", currentpath, newbasename)
			renamedCnt++
			currentpath = newpath
		}

		if !stat.IsDir() {
			continue
		}
		entries, err := os.ReadDir(currentpath)
		if err != nil {
			if !continueOnError {
				return renamed, err
			}
			log.Errorf("%q: %v", currentpath, err)
			errorCnt++
			continue
		}
		for _, entry := range entries {
			if strings.HasPrefix(entry.Name(), ".") {
				continue
			}
			pathes = append(pathes, filepath.Join(currentpath, entry.Name()))
		}
	}

	if errorCnt > 0 {
		return renamed, fmt.Errorf("%d errors", errorCnt)
	}
	return renamed, nil
}
