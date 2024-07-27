package asmrconnecting

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/jmoiron/sqlx"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/site"
	"github.com/sagan/erodownloader/site/alist"
)

// passwords:
// ["野兽先辈", "Kathleens"]

type Site struct {
	alist.AlistSite
	dir string
	db  *sqlx.DB // csvq db of dir
}

func Creator(name string, sc *config.SiteConfig, c *config.Config) (site.Site, error) {
	dir := filepath.Join(config.ConfigDir, name)
	if err := os.MkdirAll(dir, 0600); err != nil {
		return nil, fmt.Errorf("failed to create meta data dir: %w", err)
	}
	s := &Site{dir: dir}
	asite, err := alist.New(name, sc, s)
	if err != nil {
		return nil, err
	}
	s.AlistSite = *asite
	return s, nil
}

func init() {
	site.Register(&site.RegInfo{
		Name:    "asmrconnecting",
		Creator: Creator,
	})
}

var _ site.Site = (*Site)(nil)
