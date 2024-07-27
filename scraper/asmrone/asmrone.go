// Scrape dlsite asmr meta info from https://asmr.one/works
package asmrone

import (
	"fmt"
	"regexp"

	"github.com/sagan/erodownloader/scraper"
	"github.com/sagan/erodownloader/scraper/dlsite"
	"github.com/sagan/erodownloader/scraper/jsonscraper"
)

const ASMRONE = "asmrone"
const VERSION = "0.1.0"

// \b 不包含 "_"
var numberRegexp = regexp.MustCompile(`\b(?P<number>[RV]J\d{5,12})(\b|_)`)

// E.g. https://api.asmr-200.com/api/workInfo/RJ01178645
// "RJ" prefix could be omitted.
type AsmroneWorkInfo struct {
	AgeCategoryString string `json:"age_category_string,omitempty"` // "adult"
	Release           string `json:"release,omitempty"`             // "yyyy-MM-dd"
	Circle            struct {
		Name string `json:"name,omitempty"`
	} `json:"circle,omitempty"`
	Tags []struct {
		I18n struct {
			JaJp struct {
				Name string `json:"name,omitempty"`
			} `json:"ja-jp,omitempty"`
		} `json:"i18n,omitempty"`
		Name string `json:"name,omitempty"`
	} `json:"tags,omitempty"`
	MainCoverUrl string `json:"mainCoverUrl,omitempty"`
	SourceType   string `json:"source_type,omitempty"` // "DLSITE"
	SourceId     string `json:"source_id,omitempty"`   // "RJ01178645"
	Title        string `json:"title,omitempty"`
}

func (awi *AsmroneWorkInfo) GetTags() (tags []string) {
	for _, tag := range awi.Tags {
		if tag.I18n.JaJp.Name != "" {
			tags = append(tags, tag.I18n.JaJp.Name)
		} else if tag.Name != "" {
			tags = append(tags, tag.Name)
		}
	}
	if awi.AgeCategoryString == "adult" {
		tags = append(tags, scraper.TAG_R18)
	}
	return tags
}

func init() {
	jsonScraper := &jsonscraper.JsonScraper[AsmroneWorkInfo]{
		NumberRegexp: numberRegexp,
		GetRename:    scraper.GetRename,
		Source:       dlsite.DLSITE,
		GetUrl: func(number string) (url string) {
			return fmt.Sprintf("https://api.asmr-200.com/api/workInfo/%s", number)
		},
		GetData: func(data *AsmroneWorkInfo) (metadata *scraper.Metadata, err error) {
			if data == nil || data.Title == "" || data.SourceType != "DLSITE" {
				return nil, scraper.ErrNotFound
			}
			metadata = &scraper.Metadata{
				Title:  data.Title,
				Author: data.Circle.Name,
				Date:   data.Release,
				Tags:   data.GetTags(),
				Number: data.SourceId,
			}
			// trea all works in asmr.one as voices
			metadata.Tags = append(metadata.Tags, scraper.TAG_VOICEASMR)
			return metadata, nil
		},
		GetCover: func(data *AsmroneWorkInfo) (coverUrl string) {
			return data.MainCoverUrl
		},
	}
	scraper.Register(&scraper.Scraper{
		Name:    ASMRONE,
		Version: VERSION,
		Pre:     jsonScraper.Pre,
		Do:      jsonScraper.Do,
	})
}
