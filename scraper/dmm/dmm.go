package dmm

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Noooste/azuretls-client"
	"github.com/PuerkitoBio/goquery"
	"github.com/sagan/erodownloader/scraper"
	"github.com/sagan/erodownloader/scraper/html"
)

const DMM = "dmm"
const VERSION = "0.1.0"
const RJ_URL = "https://www.dlsite.com/maniax/work/=/product_id/%s.html"
const VJ_URL = "https://www.dlsite.com/pro/work/=/product_id/%s.html"

// e.g. https://www.dmm.co.jp/dc/doujin/-/detail/=/cid=d_273812/
const D_URL = "https://www.dmm.co.jp/dc/doujin/-/detail/=/cid=%s/"

var NumberRegexp = regexp.MustCompile(`\b(?P<number>d_\d{5,12})(\b|_)`)

func init() {
	htmlScraper := &html.HtmlScraper{
		UseWebArchiveFor404: true,
		Cookie:              "setover18=1; ckcy=1; is_intarnal=true; age_check_done=1;",
		TitleSelector:       "h1.productTitle__txt",
		AuthorSelector:      ".circleName__item",
		SeriesNameSelector:  `th:contains("シリーズ名") + td > a`,
		DateSelector:        `.productInformation__item:has(.informationList__ttl:contains("配信開始日")) .informationList__txt`,
		DateFormats:         []string{"2006/01/02 15:04", "2006/01/02"},
		TagsSelector:        `.productInformation__item:has(.informationList__ttl:contains("題材")) .informationList__txt a, .m-genreTag, .c_icon_productGenre`,
		// no way to detect whether is R18.
		TagMapper: map[string]string{
			"ボイス": scraper.TAG_VOICEASMR,
		},
		RemoveTags:    []string{"FANZA専売", "旧作"},
		TextSelectors: []string{".m-productSummary"},
		CoverSelector: []string{`.productPreview__item img`},
		NumberRegexp:  NumberRegexp,
		Source:        DMM,
		GetUrl: func(number string) (url string) {
			if len(number) > 2 && strings.HasPrefix(number, "d_") {
				return fmt.Sprintf(D_URL, number)
			}
			return ""
		},
		GetData: func(doc *goquery.Document, res *azuretls.Response) (metadata *scraper.Metadata, err error) {
			if strings.HasPrefix(res.Request.Url, "https://accounts.dmm.co.jp/service/login/") {
				return nil, fmt.Errorf("current ip can not access dmm")
			}
			return nil, nil
		},
		GetRename: scraper.GetRename,
	}
	scraper.Register(&scraper.Scraper{
		Name:    DMM,
		Version: VERSION,
		Pre:     htmlScraper.Pre,
		Do:      htmlScraper.Do,
	})
}
