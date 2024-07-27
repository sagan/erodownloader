package jsonscraper

import (
	"fmt"
	"path/filepath"
	"regexp"
	"slices"
	"strings"

	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/httpclient"
	"github.com/sagan/erodownloader/scraper"
	"github.com/sagan/erodownloader/util"
)

// T: the response data type, e.g. a struct.
type JsonScraper[T any] struct {
	Source       string
	GetUrl       func(number string) (url string)
	GetData      func(data *T) (metadata *scraper.Metadata, err error)
	GetCover     func(data *T) (coverUrl string)
	GetRename    func(originalname string, metadata *scraper.Metadata) (canonicalName string, shouldRename bool)
	NumberRegexp *regexp.Regexp
}

func (js *JsonScraper[T]) Pre(dirname string) bool {
	return scraper.GetNumberFromFilename(js.NumberRegexp, filepath.Base(dirname)) != ""
}

func (js *JsonScraper[T]) Do(dirname string, tmpdir string) (metadata *scraper.Metadata, err error) {
	basename := filepath.Base(dirname)
	number := scraper.GetNumberFromFilename(js.NumberRegexp, basename)
	if number == "" {
		return nil, scraper.ErrNoNumber
	}
	baseUrl := js.GetUrl(number)
	if baseUrl == "" {
		return nil, scraper.ErrNotFound
	}
	var data *T
	err = httpclient.FetchJson(baseUrl, &data, true)
	if err != nil {
		if strings.Contains(err.Error(), "status=404") {
			return nil, scraper.ErrNotFound
		}
		return nil, err
	}

	metadata, err = js.GetData(data)
	if err != nil {
		return nil, err
	}
	if metadata.Title == "" {
		return nil, scraper.ErrNotFound
	}

	var files []string
	if util.ExistsFileWithAnySuffix(filepath.Join(dirname, scraper.COVER), constants.ImgExts...) == "" {
		if coverImgUrl := js.GetCover(data); coverImgUrl != "" {
			tmpfile := filepath.Join(tmpdir, scraper.COVER)
			tmpfile, ext, err := httpclient.SaveUrl(coverImgUrl, tmpfile, true, true)
			if err != nil {
				return nil, fmt.Errorf("failed to save cover: %w", err)
			}
			if !slices.Contains(constants.ImgExts, ext) {
				return nil, fmt.Errorf("invalid cover file format: %s", ext)
			}
			files = append(files, filepath.Base(tmpfile))
		}
	}
	metadata.Files = files

	if js.GetRename != nil {
		metadata.CanonicalFilename, metadata.ShouldRename = js.GetRename(basename, metadata)
	}
	return metadata, nil
}
