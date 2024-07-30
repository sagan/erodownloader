package asmrconnecting

import (
	"database/sql"
	"errors"
	"fmt"
	"io/fs"
	"net/url"
	"os"
	"path/filepath"
	"regexp"
	"slices"
	"strconv"
	"strings"
	"time"

	"github.com/jmoiron/sqlx"
	log "github.com/sirupsen/logrus"

	_ "github.com/mithrandie/csvq-driver"
	"github.com/sagan/erodownloader/httpclient"
	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/site"
	"github.com/sagan/erodownloader/util"
)

type FILES_TYPE int

const (
	FILES_TYPE_NONE FILES_TYPE = iota
	FILES_TYPE_ORIGINAL
	FILES_TYPE_MP3
)

const INDEX_DIR = "/Directory"
const DLSITE_INDEX = "DL tree.xlsx"

type DlsiteIndexRecord struct {
	ItemNumber sql.NullString `db:"音声号码,omitempty"` // "RJ308835"
	Mega       sql.NullString `db:"MEGA链接,omitempty"`
	InputDate  sql.NullString `db:"录入日期,omitempty"`   // "2021-01-01"
	Tag        sql.NullString `db:"TAG,omitempty"`    // "治愈、胸部／奶子、淫语、双声道立体声／人头麦、学校／学园"
	Voice      sql.NullString `db:"声优,omitempty"`     // "思ちぽ  \  秋野かえで"
	ItemTitle  sql.NullString `db:"标题,omitempty"`     // "純情おま○こ当番【フォーリーサウンド】"
	Comment    sql.NullString `db:"备注,omitempty"`     // "null", "【RJ01112100】【官方汉化】", "【英文作品】", "【中英作品】"
	Type       sql.NullString `db:"类型,omitempty"`     // "音声·ASMR"
	Brand      sql.NullString `db:"社团,omitempty"`     // "青春×フェティシズム"
	SellDate   sql.NullString `db:"销售日期,omitempty"`   // "2021-01-01"
	UpdateDate sql.NullString `db:"最后修改日期,omitempty"` // "2021-01-01"
	ItemSize   sql.NullString `db:"原档容量,omitempty"`   // "5.61 GB"
	SizeMp3    sql.NullString `db:"MP3容量,omitempty"`  // "260.11 MB"
	Folder1    sql.NullString `db:"原档标注一,omitempty"`  // "03 - 同步完成合集"
	Folder2    sql.NullString `db:"原档标注二,omitempty"`  // "Z - 0088"
	FolderMp3  sql.NullString `db:"MP3标注一,omitempty"` // "01 - 同步MP3完成合集"
	FolderMp32 sql.NullString `db:"MP3标注二,omitempty"` // "M - 0030"
	site       string
}

// Time implements site.Resource.
func (d *DlsiteIndexRecord) Time() int64 {
	date := ""
	if notNull(d.UpdateDate.String) {
		date = d.UpdateDate.String
	} else if notNull(d.SellDate.String) {
		date = d.SellDate.String
	}
	if date != "" {
		if t, err := time.Parse("2006-01-02", date); err == nil {
			return t.Unix()
		}
	}
	return 0
}

func (d *DlsiteIndexRecord) Author() string {
	return d.Brand.String
}

// Folder1: "47 - 同步完成合集", Folder2: "C - 0285", Number: "RJ01106734".
// path: "DLsite/08/08 - 同步完成合集/C - 0285/RJ01106734".
func (d *DlsiteIndexRecord) Id() string {
	var folder1, folder2 string
	filesType := d.filesType()
	isMp3 := false
	switch filesType {
	case FILES_TYPE_ORIGINAL:
		folder1 = d.Folder1.String
		folder2 = d.Folder2.String
	case FILES_TYPE_MP3:
		folder1 = d.FolderMp3.String
		folder2 = d.FolderMp32.String
		isMp3 = true
	default:
		return ""
	}
	indexNumberStr, _, found := strings.Cut(folder1, " ")
	if !found {
		return ""
	}
	folder := ""
	if isMp3 {
		folder = fmt.Sprintf("/DLsite/MP3/%s/%s/%s", folder1, folder2, d.ItemNumber.String)
	} else {
		folder = fmt.Sprintf("/DLsite/%s/%s/%s/%s", indexNumberStr, folder1, folder2, d.ItemNumber.String)
	}
	values := url.Values{}
	values.Set("path", folder)
	values.Set("number", d.ItemNumber.String)
	values.Set("type", "resource")
	if d.site != "" {
		values.Set("site", d.site)
	}
	return values.Encode()
}

func (d *DlsiteIndexRecord) Title() string {
	return d.ItemTitle.String
}

func (d *DlsiteIndexRecord) Number() string {
	return d.ItemNumber.String
}

func (d *DlsiteIndexRecord) Site() string {
	return d.site
}

func (d *DlsiteIndexRecord) filesType() (t FILES_TYPE) {
	if notNull(d.ItemNumber.String) {
		if notNull(d.Folder1.String) && notNull(d.Folder2.String) {
			t = FILES_TYPE_ORIGINAL
		} else if notNull(d.FolderMp3.String) && notNull(d.FolderMp32.String) {
			t = FILES_TYPE_MP3
		}
	}
	return
}

// Size implements site.Resource.
func (d *DlsiteIndexRecord) Size() (size int64) {
	t := d.filesType()
	switch t {
	case FILES_TYPE_ORIGINAL:
		size, _ = util.RAMInBytes(d.ItemSize.String)
	case FILES_TYPE_MP3:
		size, _ = util.RAMInBytes(d.SizeMp3.String)
	}
	return
}

var splitterRegexp = regexp.MustCompile(`\s*[\\,、]\s*`)

// Tags implements site.Resource.
func (d *DlsiteIndexRecord) Tags() (tags schema.Tags) {
	if d.filesType() == FILES_TYPE_MP3 {
		tags = append(tags, "mp3")
	}
	if notNull(d.Type.String) {
		tags = append(tags, "genre:"+d.Type.String)
	}
	if notNull(d.Tag.String) {
		tags = append(tags, splitterRegexp.Split(d.Tag.String, -1)...)
	}
	if notNull(d.Voice.String) {
		for _, voice := range splitterRegexp.Split(d.Voice.String, -1) {
			tags = append(tags, "narrator:"+voice)
		}
	}
	tags = util.UniqueSlice(tags)
	slices.Sort(tags)
	return
}

func (s *Site) FindResourceFiles(id string) (files site.Files, err error) {
	files, err = s.ReadDir(id)
	if err != nil {
		return nil, err
	}
	files = util.FilterSlice(files, func(f site.File) bool {
		// ignore .par2 files: https://asmrconnecting.xyz/Repair
		return !f.IsDir() && !strings.HasSuffix(f.Name(), ".par2")
	})
	return files, nil
}

func (s *Site) FindResources(qs string) (resources site.Resources, err error) {
	query, err := url.ParseQuery(qs)
	if err != nil {
		return nil, fmt.Errorf("malformed qs: %w", err)
	}
	indexFile := DLSITE_INDEX + ".csv"
	if s.db == nil {
		if !util.FileExists(filepath.Join(s.dir, indexFile)) || !query.Has("cache") {
			if err := s.downloadResourceIndexes(); err != nil {
				return nil, err
			}
		}
		if s.db, err = sqlx.Open("csvq", s.dir); err != nil {
			return nil, fmt.Errorf("failed to create index db: %w", err)
		}
	}

	records := []*DlsiteIndexRecord{}
	sql := fmt.Sprintf(`SELECT
音声号码, MEGA链接, 录入日期, TAG, 声优, 标题, 备注, 类型, 社团, 销售日期, 最后修改日期, 原档容量, MP3容量,
原档标注一, 原档标注二, MP3标注一, MP3标注二
FROM %s WHERE `, fmt.Sprintf("`%s`", indexFile))
	var args []any
	if rawquery := query.Get("raw"); rawquery != "" {
		sql += rawquery
	} else {
		sql += "1 = 1 "
		if number := query.Get("number"); number != "" {
			sql += "and 音声号码 = ? "
			args = append(args, number)
		}
		if author := query.Get("author"); author != "" {
			sql += "and 社团 = ? "
			args = append(args, author)
		}
		if genre := query.Get("genre"); genre != "" {
			sql += "and 类型 = ? "
			args = append(args, genre)
		}
		if query.Has("q") {
			for _, q := range query["q"] {
				sql += `and 标题 like ? `
				args = append(args, `%`+q+`%`)
			}
		}
		if query.Has("narrator") {
			deli := `(^|$|\s|\\|/)` // for some reason, "_b" will not work.
			for _, narrator := range query["narrator"] {
				sql += `and REGEXP_MATCH(声优, ?) `
				args = append(args, deli+regexp.QuoteMeta(narrator)+deli)
			}
		}
		if query.Has("tag") {
			for _, tag := range query["tag"] {
				sql += `and TAG like ? `
				args = append(args, `%`+tag+`%`)
			}
		}
		if whereSql := query.Get("where"); whereSql != "" {
			whereSql += fmt.Sprintf(`and ( %s ) `, whereSql)
		}
		if orders := query["order"]; len(orders) > 0 {
			orderSql := ""
			deli := ""
			for _, order := range orders {
				field := ""
				desc := false
				if strings.HasPrefix(order, "-") {
					field = order[1:]
					desc = true
				} else {
					field = order
				}
				if field == "" {
					return nil, fmt.Errorf("invalid order qs (can not be empty)")
				}
				orderSql += deli + field
				if desc {
					orderSql += " desc"
				}
				deli = ", "
			}
			sql += " order by " + orderSql
		}
		if query.Has("limit") {
			limit, err := strconv.Atoi(query.Get("limit"))
			if err != nil || limit <= 0 {
				return nil, fmt.Errorf("invalid limit qs: not a valid int (err=%w)", err)
			}
			sql += " limit " + fmt.Sprint(limit)
		}
	}
	log.Tracef("sql: %s, args: %v", sql, args)
	if err := s.db.Select(&records, sql, args...); err != nil {
		return nil, err
	}
	for _, record := range records {
		record.site = s.Name
		resources = append(resources, record)
	}
	return resources, nil
}

func (s *Site) downloadResourceIndexes() (err error) {
	indexFiles, err := s.Search("path=" + url.QueryEscape(INDEX_DIR))
	if err != nil {
		return fmt.Errorf("failed to get index files: %w", err)
	}
	var indexFilenames []string
	for _, indexFile := range indexFiles {
		if indexFile.IsDir() || indexFile.Name() != DLSITE_INDEX {
			continue
		}
		doDownload := false
		localfile := filepath.Join(s.dir, indexFile.Name())
		if stat, err := os.Stat(localfile); errors.Is(err, fs.ErrNotExist) {
			doDownload = true
		} else if err == nil && indexFile.Time()-stat.ModTime().Unix() > 30*60 {
			doDownload = true
		} else {
			indexFilenames = append(indexFilenames, indexFile.Name())
			log.Tracef("index file %q exists in local", indexFile.Name())
		}
		if !doDownload {
			continue
		}
		if indexFile.RawUrl() == "" {
			if indexFile, err = s.GetFile(indexFile.Id()); err != nil {
				log.Tracef("failed to get file details: %v", err)
				continue
			}
		}
		if indexFile.RawUrl() == "" {
			log.Tracef("file %q url is empty: %v", indexFile.Name(), err)
			continue
		}
		res, err := httpclient.FetchUrl(indexFile.RawUrl(), nil, true)
		if err != nil {
			log.Tracef("failed to download index file: %v", err)
			continue
		}
		if err := os.WriteFile(localfile, res.Body, 0600); err != nil {
			log.Tracef("failed to write local file: %v", err)
			continue
		}
		indexFilenames = append(indexFilenames, indexFile.Name())
	}
	log.Tracef("index files: %v", indexFilenames)
	if err := s.parpareResourceIndexes(indexFilenames); err != nil {
		return fmt.Errorf("failed to prepare indexes: %w", err)
	}
	return nil
}

func (s *Site) parpareResourceIndexes(indexFiles []string) (err error) {
	for _, indexFile := range indexFiles {
		if strings.HasSuffix(indexFile, "xlsx") || strings.HasSuffix(indexFile, "xls") {
			xlsFile := filepath.Join(s.dir, indexFile)
			csvfile := filepath.Join(s.dir, indexFile+".csv")
			xlsFileStat, err := os.Stat(xlsFile)
			if err != nil {
				return nil
			}
			if csvStat, err := os.Stat(csvfile); err == nil && csvStat.ModTime().Unix() >= xlsFileStat.ModTime().Unix() {
				continue
			}
			if err := util.Xlsx2Csv(xlsFile); err != nil {
				log.Tracef("failed to generate csv: %v", err)
				return err
			}
		}
	}
	return nil
}

var _ site.Resource = (*DlsiteIndexRecord)(nil)

func isNull(str string) bool {
	return str == "" || str == "null"
}

func notNull(str string) bool {
	return !isNull(str)
}
