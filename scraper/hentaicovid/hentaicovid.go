// https://hentaicovid.com/index.php/voices-asmr/
package hentaicovid

import (
	"fmt"
	"strings"

	"github.com/Noooste/azuretls-client"
	"github.com/PuerkitoBio/goquery"
	"github.com/sagan/erodownloader/scraper"
	"github.com/sagan/erodownloader/scraper/dlsite"
	"github.com/sagan/erodownloader/scraper/html"
)

const NAME = "hentaicovid"
const VERSION = "0.1.0"

func init() {
	htmlScraper := &html.HtmlScraper{
		NoRedirect:          true,
		UseWebArchiveFor404: false,
		AuthorSelector:      `#work_maker li:contains("Author/サークル名:") span`,
		DateSelector:        `#work_maker li:contains("Released/販売日:") span`,
		DateFormats:         []string{"2006年01月02日", "2006年01月02日 15時"},
		TextSelectors:       []string{".work_parts.type_text"},
		CoverSelector: []string{
			`#content-detial img`, // first image in content block
		},
		TagsSelector: `#work_maker li:contains("Genre/ジャンル:") a`,
		SelectorTags: map[string]string{
			`#work_maker span:contains("` + scraper.TAG_R18 + `")`:       scraper.TAG_R18,
			`#work_maker span:contains("` + scraper.TAG_VOICEASMR + `")`: scraper.TAG_VOICEASMR,
			`#work_maker span:contains("Voices・ASMR")`:                   scraper.TAG_VOICEASMR,
		},
		NumberRegexp: dlsite.RjNumberRegexp,
		Source:       dlsite.DLSITE,
		GetUrl: func(number string) (url string) {
			if len(number) > 2 && strings.HasPrefix(number, "RJ") {
				return fmt.Sprintf("https://hentaicovid.com/index.php/voices-asmr/free-download-%s", number[2:])
			}
			return ""
		},
		GetData: func(doc *goquery.Document, res *azuretls.Response) (metadata *scraper.Metadata, err error) {
			titleTxt := doc.Find(`#breadcrumb h2`).First().Text()
			title, number, found := strings.Cut(titleTxt, "|")
			title = strings.TrimSpace(title)
			number = strings.TrimSpace(number)
			if !found || !strings.HasPrefix(number, "RJ") {
				return nil, scraper.ErrNotFound
			}
			return &scraper.Metadata{
				Title:  title,
				Number: number,
			}, nil
		},
		GetRename: scraper.GetRename,
		TagMapper: dlsite.TagMapper,
	}
	scraper.Register(&scraper.Scraper{
		Name:    NAME,
		Version: VERSION,
		Pre:     htmlScraper.Pre,
		Do:      htmlScraper.Do,
	})
}
