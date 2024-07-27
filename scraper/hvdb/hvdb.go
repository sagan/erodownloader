// https://hvdb.me/Dashboard/Details/01201812
// Pure-english.
package hvdb

import (
	"fmt"
	"strings"

	"github.com/Noooste/azuretls-client"
	"github.com/PuerkitoBio/goquery"
	"github.com/sagan/erodownloader/scraper"
	"github.com/sagan/erodownloader/scraper/dlsite"
	"github.com/sagan/erodownloader/scraper/html"
)

const NAME = "hvdb"
const VERSION = "0.1.0"

func init() {
	htmlScraper := &html.HtmlScraper{
		NoRedirect:          true,
		UseWebArchiveFor404: true,
		TitleSelector:       `.infoLabel`,
		SelectorTags: map[string]string{
			`html`: scraper.TAG_VOICEASMR, // it's ugly
		},
		OnError: func(res *azuretls.Response, err error) error {
			if res != nil && res.StatusCode == 500 {
				return scraper.ErrNotFound
			}
			return err
		},
		CoverSelector: []string{`img.detailImage`},
		NumberRegexp:  dlsite.RjNumberRegexp,
		Source:        dlsite.DLSITE,
		GetUrl: func(number string) (url string) {
			if len(number) > 2 && strings.HasPrefix(number, "RJ") {
				return fmt.Sprintf("https://hvdb.me/Dashboard/Details/%s", number[2:])
			}
			return ""
		},
		GetData: func(doc *goquery.Document, res *azuretls.Response) (metadata *scraper.Metadata, err error) {
			circle := doc.Find(`.detailCircle`).First().Text()
			// e.g.: "巨乳大好き屋 / big boobs lover"
			circle, _, _ = strings.Cut(circle, `/`)
			circle = strings.TrimSpace(circle)
			return &scraper.Metadata{
				Author: circle,
			}, nil
		},
		GetRename: scraper.GetRename,
	}
	scraper.Register(&scraper.Scraper{
		Name:    NAME,
		Version: VERSION,
		Pre:     htmlScraper.Pre,
		Do:      htmlScraper.Do,
	})
}
