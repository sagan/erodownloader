package html

import (
	"bytes"
	"fmt"
	"net/http"
	"net/url"
	"path/filepath"
	"regexp"
	"slices"
	"strings"
	"time"

	"github.com/Noooste/azuretls-client"
	"github.com/PuerkitoBio/goquery"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/httpclient"
	"github.com/sagan/erodownloader/scraper"
	"github.com/sagan/erodownloader/util"
	"github.com/sagan/erodownloader/util/stringutil"
)

type HtmlScraper struct {
	IgnoreErrors        bool
	NoRedirect          bool
	UseWebArchiveFor404 bool
	Cookie              string
	TitleSelector       string
	AuthorSelector      string
	SeriesNameSelector  string
	DateSelector        string
	DateFormats         []string
	NarratorSelector    string
	TagsSelector        string
	CoverSelector       []string
	TextSelectors       []string          // all text sections will be joined by "\n\n"
	NumberRegexp        *regexp.Regexp    // Use "number" as group name
	SelectorTags        map[string]string // selector => tag. If selector element exists, add that tag
	TagMapper           map[string]string // old tag => new tag
	RemoveTags          []string
	Source              string
	GetData             func(doc *goquery.Document, res *azuretls.Response) (metadata *scraper.Metadata, err error)
	GetUrl              func(number string) (url string)
	GetRename           func(originalname string, metadata *scraper.Metadata) (canonicalName string, shouldRename bool)
	GetCoverSelector    func(res *azuretls.Response, doc *goquery.Document) []string
	OnError             func(res *azuretls.Response, err error) error
}

// Doc: https://archive.org/help/wayback_api.php .
// E.g.: http://archive.org/wayback/available?url=example.com .
type WebArchiveAvailableData struct {
	Url               string `json:"url,omitempty"`
	ArchivedSnapshots struct {
		Closest *struct {
			Available bool   `json:"available,omitempty"`
			Url       string `json:"url,omitempty"`
			Timestamp string `json:"timestamp,omitempty"`
			Status    string `json:"status,omitempty"`
		} `json:"closest,omitempty"`
	} `json:"archived_snapshots,omitempty"`
}

func (hs *HtmlScraper) Pre(dirname string) bool {
	return scraper.GetNumberFromFilename(hs.NumberRegexp, filepath.Base(dirname)) != ""
}

func (hs *HtmlScraper) Do(dirname string, tmpdir string) (*scraper.Metadata, error) {
	basename := filepath.Base(dirname)
	number := scraper.GetNumberFromFilename(hs.NumberRegexp, basename)
	if number == "" {
		return nil, scraper.ErrNoNumber
	}
	baseUrl := hs.GetUrl(number)
	if baseUrl == "" {
		return nil, scraper.ErrNotFound
	}
	baseUrlObj, err := url.Parse(baseUrl)
	if err != nil {
		return nil, err
	}
	var header http.Header
	if hs.Cookie != "" {
		header = http.Header{"Cookie": []string{hs.Cookie}}
	}
	res, err := httpclient.FetchUrl(baseUrl, header, true)
	if hs.NoRedirect {
		if res != nil && res.Request.Url != baseUrl {
			return nil, scraper.ErrNotFound
		}
	}
	isWebArchive := false
	if err != nil {
		(func() {
			var err1 error
			if !strings.Contains(err.Error(), "status=404") || !hs.UseWebArchiveFor404 {
				return
			}
			// Web archive 的 API 有毛病。有的 url 必须不 escape 才有结果，有的 url 则正好相反。
			webarchiveUrls := []string{
				"http://archive.org/wayback/available?url=" + baseUrl,
				"http://archive.org/wayback/available?url=" + url.QueryEscape(baseUrl),
			}
			for _, webarchiveUrl := range webarchiveUrls {
				var webarchiveData *WebArchiveAvailableData
				err1 = httpclient.FetchJson(webarchiveUrl, &webarchiveData, false)
				if err1 != nil {
					return
				}
				if webarchiveData.ArchivedSnapshots.Closest == nil || !webarchiveData.ArchivedSnapshots.Closest.Available ||
					webarchiveData.ArchivedSnapshots.Closest.Status != "200" {
					continue
				}
				res1, err1 := httpclient.FetchUrl(webarchiveData.ArchivedSnapshots.Closest.Url, nil, true)
				if err1 == nil {
					res = res1
					isWebArchive = true
					err = nil
				}
			}
		})()
	}
	if err != nil {
		if hs.OnError != nil {
			if err = hs.OnError(res, err); err == nil {
				err = scraper.ErrNotFound
			}
			return nil, err
		} else if hs.IgnoreErrors || strings.Contains(err.Error(), "status=404") {
			return nil, scraper.ErrNotFound
		}
		return nil, err
	}
	doc, err := goquery.NewDocumentFromReader(bytes.NewReader(res.Body))
	if err != nil {
		return nil, err
	}
	title := ""
	author := ""
	seriesName := ""
	date := ""
	text := ""
	var narrators []string
	var tags []string
	var files []string

	if hs.TitleSelector != "" {
		if str := stringutil.Clean(doc.Find(hs.TitleSelector).First().Text()); str != "" {
			title = str
		}
	}

	if hs.AuthorSelector != "" {
		if str := stringutil.Clean(doc.Find(hs.AuthorSelector).First().Text()); str != "" {
			author = str
		}
	}

	if hs.SeriesNameSelector != "" {
		if str := stringutil.Clean(doc.Find(hs.SeriesNameSelector).First().Text()); str != "" {
			seriesName = str
		}
	}

	if hs.DateSelector != "" {
		if str := stringutil.Clean(doc.Find(hs.DateSelector).First().Text()); str != "" {
			for _, format := range hs.DateFormats {
				if time, err := time.Parse(format, str); err == nil {
					date = time.Format("2006-01-02")
					break
				}
			}
		}
	}

	if hs.NarratorSelector != "" {
		doc.Find(hs.NarratorSelector).Each(func(i int, s *goquery.Selection) {
			narrator := stringutil.Clean(s.Text())
			if narrator != "" {
				narrators = append(narrators, narrator)
			}
		})
	}

	if hs.TagsSelector != "" {
		doc.Find(hs.TagsSelector).Each(func(i int, s *goquery.Selection) {
			tag := stringutil.Clean(s.Text())
			if tag != "" {
				tags = append(tags, tag)
			}
		})
	}
	for selector, tag := range hs.SelectorTags {
		if doc.Find(selector).Length() > 0 {
			tags = append(tags, tag)
		}
	}

	if len(hs.TextSelectors) > 0 {
		var texts []string
		for _, selector := range hs.TextSelectors {
			if text := util.DomSelectionText(doc.Find(selector)); text != "" {
				texts = append(texts, text)
			}
		}
		text = strings.Join(texts, "\n\n")
	}

	if util.ExistsFileWithAnySuffix(filepath.Join(dirname, scraper.COVER), constants.ImgExts...) == "" {
		var coverSelectors []string
		if hs.GetCoverSelector != nil {
			coverSelectors = hs.GetCoverSelector(res, doc)
		}
		coverSelectors = append(coverSelectors, hs.CoverSelector...)
		srcAttrs := []string{"src", "srcset", "data-src"}
		var coverImgURL *url.URL
		for _, selector := range coverSelectors {
			selector = strings.ReplaceAll(selector, "{{number}}", url.QueryEscape(number))
			s := doc.Find(selector)
			if s.Length() == 0 {
				continue
			}
			imgUrl := ""
			for _, srcAttr := range srcAttrs {
				if v := s.AttrOr(srcAttr, ""); v != "" {
					imgUrl = v
					break
				}
			}
			if imgUrl == "" {
				continue
			}
			urlObj, err := url.Parse(imgUrl)
			if err != nil {
				continue
			}
			coverImgURL = baseUrlObj.ResolveReference(urlObj)
			break
		}
		if coverImgURL != nil {
			coverImgUrl := coverImgURL.String()
			tmpfile := filepath.Join(tmpdir, scraper.COVER)
			tmpfile, ext, err := httpclient.SaveUrl(coverImgUrl, tmpfile, true, true)
			if err != nil {
				if isWebArchive {
					return nil, scraper.ErrNotFound
				}
				return nil, fmt.Errorf("failed to save cover: %w", err)
			}
			if !slices.Contains(constants.ImgExts, ext) {
				return nil, fmt.Errorf("invalid cover file format: %s", ext)
			}
			files = append(files, filepath.Base(tmpfile))
		}
	}

	var metadata *scraper.Metadata
	if hs.GetData != nil {
		metadata, err = hs.GetData(doc, res)
		if err != nil {
			return nil, err
		}
	}
	if metadata == nil {
		metadata = &scraper.Metadata{}
	}

	util.Assign(metadata, &scraper.Metadata{
		Title:    title,
		Number:   number,
		Author:   author,
		Date:     date,
		Series:   seriesName,
		Tags:     tags,
		Narrator: narrators,
		Source:   hs.Source,
		Text:     text,
		Files:    files,
	}, nil)

	if hs.TagMapper != nil {
		metadata.Tags = util.Map(metadata.Tags, func(tag string) string {
			if tag, ok := hs.TagMapper[tag]; ok {
				return tag
			}
			return tag
		})
	}
	if len(hs.RemoveTags) > 0 {
		metadata.Tags = util.FilterSlice(metadata.Tags, func(t string) bool { return !slices.Contains(hs.RemoveTags, t) })
	}

	if hs.GetRename != nil {
		metadata.CanonicalFilename, metadata.ShouldRename = hs.GetRename(basename, metadata)
	}
	return metadata, nil
}
