package dlsite

import (
	"fmt"
	"regexp"
	"strings"

	"github.com/Noooste/azuretls-client"
	"github.com/PuerkitoBio/goquery"
	"github.com/sagan/erodownloader/scraper"
	"github.com/sagan/erodownloader/scraper/html"
)

const DLSITE = "dlsite"
const VERSION = "0.1.0"
const RJ_URL = "https://www.dlsite.com/maniax/work/=/product_id/%s.html"
const VJ_URL = "https://www.dlsite.com/pro/work/=/product_id/%s.html"

// BJ: BL / GL works.
// For GL works, will get redirected to https://www.dlsite.com/girls-pro/work/=/product_id/%s.html
const BJ_URL = "https://www.dlsite.com/bl-pro/work/=/product_id/%s.html"

// 1. 将一些外语 tag 转回日语，因为某些刮削器可能会刮到这些标签。（workaround）
// 2. dlsite 标签反和谐。数据来源：kamept。
var TagMapper = map[string]string{
	"Voz / ASMR":    scraper.TAG_VOICEASMR,
	"Voz・ASMR":      scraper.TAG_VOICEASMR,
	"Voice / ASMR":  scraper.TAG_VOICEASMR,
	"Voice・ASMR":    scraper.TAG_VOICEASMR,
	"Voices / ASMR": scraper.TAG_VOICEASMR,
	"Voices・ASMR":   scraper.TAG_VOICEASMR,
	"Audio / ASMR":  scraper.TAG_VOICEASMR,
	"Audio・ASMR":    scraper.TAG_VOICEASMR,
	"18+":           scraper.TAG_R18,
	"+18":           scraper.TAG_R18,

	"ざぁ～こ♡":      "メスガキ",
	"合意なし":       "レイプ",
	"ひよこ":        "ロリ",
	"つるぺた":       "ロリ",
	"ひよこババア":     "ロリババア",
	"つるぺたババア":    "ロリババア",
	"閉じ込め":       "監禁",
	"超ひどい":       "鬼畜",
	"逆レ":         "逆レイプ",
	"命令/無理矢理":    "強制/無理矢理",
	"近親もの":       "近親相姦",
	"責め苦":        "拷問",
	"トランス/暗示":    "催眠",
	"動物なかよし":     "獣姦",
	"畜えち":        "獣姦",
	"精神支配":       "洗脳",
	"秘密さわさわ":     "痴漢",
	"しつけ":        "調教",
	"下僕":         "奴隷",
	"屈辱":         "凌辱",
	"回し":         "輪姦",
	"虫えっち":       "蟲姦",
	"モブおじさん":     "モブ姦",
	"異種えっち":      "異種姦",
	"機械責め":       "機械姦",
	"すやすやえっち":    "睡眠姦",
	"トランス/暗示ボイス": "睡眠音声",
}

// \b 不匹配 "_"
var NumberRegexp = regexp.MustCompile(`\b(?P<number>[BRV]J\d{5,12})(\b|_)`)

var RjNumberRegexp = regexp.MustCompile(`\b(?P<number>RJ\d{5,12})(\b|_)`)

func init() {
	htmlScraper := &html.HtmlScraper{
		UseWebArchiveFor404: true,
		TitleSelector:       "#work_name",
		AuthorSelector:      ".maker_name",
		SeriesNameSelector:  `th:contains("シリーズ名") + td > a`,
		DateSelector:        `tr > th:contains("販売日") + td > a`,
		DateFormats:         []string{"2006年01月02日", "2006年01月02日 15時"},
		NarratorSelector:    `tr > th:contains("声優") + td > a`,
		TagsSelector:        ".main_genre > a, .main_genre > span, .work_genre > a, .work_genre > span",
		TextSelectors:       []string{".work_parts_container"}, // ".work_parts.type_text"
		CoverSelector: []string{
			`source[srcset$="/{{number}}_img_main.webp"]`,
			`source[srcset$="/{{number}}_img_main.png"]`,
			`source[srcset$="/{{number}}_img_main.jpg"]`,
			`div[data-src$="/{{number}}_img_main.jpg"]`,
		},
		GetCoverSelector: func(res *azuretls.Response, doc *goquery.Document) []string {
			// 部分翻译作品页面，例如：
			// https://www.dlsite.com/maniax/work/=/product_id/RJ01076757.html
			// 其中里的封面图片 url 的 RJ 号是主作品的。
			// @todo: 改进对这类翻译作品的刮削。
			metaImg := doc.Find(`meta[property="og:image"]`).AttrOr("content", "")
			if matchNumber := NumberRegexp.FindAllStringSubmatch(metaImg, -1); matchNumber != nil {
				number := matchNumber[len(matchNumber)-1][NumberRegexp.SubexpIndex("number")]
				return []string{
					fmt.Sprintf(`source[srcset$="/%s_img_main.webp"]`, number),
					fmt.Sprintf(`source[srcset$="/%s_img_main.png"]`, number),
					fmt.Sprintf(`source[srcset$="/%s_img_main.jpg"]`, number),
					fmt.Sprintf(`div[srcset$="/%s_img_main.webp"]`, number),
				}
			}
			return nil
		},
		NumberRegexp: NumberRegexp,
		Source:       DLSITE,
		GetUrl: func(number string) (url string) {
			if strings.HasPrefix(number, "R") {
				return fmt.Sprintf(RJ_URL, number)
			} else if strings.HasPrefix(number, "V") {
				return fmt.Sprintf(VJ_URL, number)
			} else if strings.HasPrefix(number, "B") {
				return fmt.Sprintf(BJ_URL, number)
			}
			return ""
		},
		GetData: func(doc *goquery.Document, res *azuretls.Response) (metadata *scraper.Metadata, err error) {
			metadata = &scraper.Metadata{}
			doc.Find(`.work_edition_linklist a:not(.current)`).Each(func(i int, s *goquery.Selection) {
				if match := RjNumberRegexp.FindStringSubmatch(s.AttrOr("href", "")); match != nil {
					metadata.OtherEditionNumber = append(metadata.OtherEditionNumber, match[RjNumberRegexp.SubexpIndex("number")])
				}
			})
			return metadata, nil
		},
		GetRename: scraper.GetRename,
		TagMapper: TagMapper,
	}
	scraper.Register(&scraper.Scraper{
		Name:    DLSITE,
		Version: VERSION,
		Pre:     htmlScraper.Pre,
		Do:      htmlScraper.Do,
	})
}
