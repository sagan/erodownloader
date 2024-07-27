package config

import (
	"fmt"
	"net/http"
	"net/url"
	"os"
	"path/filepath"
	"slices"
	"strings"
	"sync"

	fhttp "github.com/Noooste/fhttp"
	log "github.com/sirupsen/logrus"
	"github.com/spf13/viper"
	"gorm.io/gorm"

	"github.com/sagan/erodownloader/schema"
	"github.com/sagan/erodownloader/util"
)

const DEFAULT_PORT = 6968 // 'E' (0x69) + 'D' (0x68)
const DEFAULT_ZIPMODE = 1 // Zip filename encoding detection mode. 0 -  strict; 1 - guess the best (shift_jis > gbk)

type Config struct {
	Sites        []*SiteConfig
	Clients      []*ClientConfig
	Passwords    []string
	SavePath     string // default save path for internal (local) clients
	Aria2Url     string // url of default local aria2 client
	Aria2Token   string // token of default local aria2 client
	FlareSolverr string // 'http://localhost:8191/v1'
	UserAgent    string
	Port         int             // web ui port
	Token        string          //web ui token
	Cookies      []*fhttp.Cookie // used keys: name, value, domain, path
}

type SiteConfig struct {
	Name     string
	Aliases  []string
	Type     string
	Url      string
	Internal bool
	Comment  string
}

type ClientConfig struct {
	Name     string
	Type     string // "aria2"
	SavePath string
	Url      string // "http://localhost:6800/jsonrpc"
	Token    string // secret or authorization token
	Internal bool
	Local    bool // the client is running in the same machine & fs as this tool
	Windows  bool // Windows use "\" as sep.
	Comment  string
}

var (
	mu                sync.Mutex
	Test1             = false
	Test2             = false
	VerboseLevel      = 0
	DefaultConfigFile = ""
	ConfigDir         = "" // "/root/.config/erodownloader"
	ConfigFilename    = "" // "erodownloader.toml"
	ConfigFile        = "" // Fullpath, e.g. "/root/.config/erodownloader/erodownloader.toml"
	ConfigName        = "" // "erodownloader"
	ConfigType        = "" // "toml"
	Data              *Config
	Db                *gorm.DB

	sitesConfigMap           = map[string]*SiteConfig{}
	internalSitesConfigMap   = map[string]*SiteConfig{}
	clientsConfigMap         = map[string]*ClientConfig{}
	internalClientsConfigMap = map[string]*ClientConfig{}
)

func (siteConfig *SiteConfig) GetName() string {
	name := siteConfig.Name
	if name == "" {
		name = siteConfig.Type
	}
	return name
}

// Return effective cookie header ("a=1; b=2" format) of provided urlObj.
// Currently, cookie path is ignored.
func (c *Config) GetCookieHeader(urlObj *url.URL) string {
	urlDomain := urlObj.Hostname()
	cookieStr := ""
	cookieSep := ""
	for _, cookie := range c.Cookies {
		cookieDomain := strings.TrimPrefix(cookie.Domain, ".")
		if urlDomain == cookieDomain || strings.HasSuffix(urlDomain, "."+cookieDomain) {
			purecookie := &http.Cookie{
				Name:  cookie.Name,
				Value: cookie.Value,
			}
			cookieStr += cookieSep + purecookie.String()
			cookieSep = "; "
			break
		}
	}
	return cookieStr
}

func Load() (err error) {
	ConfigDir = filepath.Dir(ConfigFile)
	ConfigFilename = filepath.Base(ConfigFile)
	configExt := filepath.Ext(ConfigFilename)
	ConfigName = ConfigFilename[:len(ConfigFilename)-len(configExt)]
	if configExt != "" {
		ConfigType = configExt[1:]
	}
	os.MkdirAll(ConfigDir, 0700)
	viper.SetDefault("port", DEFAULT_PORT)
	viper.SetConfigName(ConfigName)
	viper.SetConfigType(ConfigType)
	viper.AddConfigPath(ConfigDir)
	log.Infof("load config file: %s", ConfigFile)
	if err = viper.ReadInConfig(); err != nil { // file does NOT exists
		log.Infof("Fail to read config file: %v", err)
	} else if err = viper.Unmarshal(&Data); err != nil {
		log.Errorf("Fail to parse config file: %v", err)
	}
	if err != nil {
		Data = &Config{}
	}
	for _, sc := range Data.Sites {
		if sitesConfigMap[sc.GetName()] != nil {
			log.Fatalf("Invalid config file: duplicate site name %s found", sc.GetName())
		}
		sitesConfigMap[sc.GetName()] = sc
	}
	for _, cc := range Data.Clients {
		if clientsConfigMap[cc.Name] != nil {
			log.Fatalf("Invalid config file: duplicate client name %s found", cc.Name)
		}
		clientsConfigMap[cc.Name] = cc
	}
	for _, clientConfig := range InternalClients {
		if Data.SavePath != "" {
			clientConfig.SavePath = Data.SavePath
		}
		if clientConfig.Type == "aria2" {
			if Data.Aria2Url != "" {
				clientConfig.Url = Data.Aria2Url
			}
			if Data.Aria2Token != "" {
				clientConfig.Token = Data.Aria2Token
			}
		}
	}

	Db, err = schema.Init(filepath.Join(ConfigDir, "data.db"), VerboseLevel)
	if err != nil {
		return fmt.Errorf("failed to open data.db: %w", err)
	}
	return nil
}

func GetSiteConfig(name string) *SiteConfig {
	if name == "" {
		return nil
	}
	if sitesConfigMap[name] != nil {
		return sitesConfigMap[name]
	}
	return internalSitesConfigMap[name]
}

func GetClientConfig(name string) *ClientConfig {
	if name == "" {
		return nil
	}
	if clientsConfigMap[name] != nil {
		return clientsConfigMap[name]
	}
	return internalClientsConfigMap[name]
}

func UpdateCookies(userAgent string, cookies []*fhttp.Cookie) error {
	mu.Lock()
	defer mu.Unlock()
	Data.UserAgent = userAgent
	for _, cookie := range cookies {
		i := slices.IndexFunc(Data.Cookies, func(c *fhttp.Cookie) bool {
			return c.Domain == cookie.Domain && c.Path == cookie.Path && c.Name == cookie.Name
		})
		if i != -1 {
			Data.Cookies[i].Value = cookie.Value
			continue
		}
		Data.Cookies = append(Data.Cookies, cookie)
	}
	viper.Set("useragent", Data.UserAgent)
	viper.Set("cookies", Data.Cookies)
	return viper.WriteConfig()
}

func init() {
	DefaultConfigFile = filepath.Join(util.Unwrap(os.UserHomeDir()), ".config", "erodownloader", "erodownloader.toml")
}
