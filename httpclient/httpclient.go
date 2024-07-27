package httpclient

import (
	"context"
	"encoding/json"
	"errors"
	"fmt"
	"io"
	"net/http"
	"net/url"
	"os"
	"path"
	"slices"
	"strings"
	"time"

	"github.com/Noooste/azuretls-client"
	fhttp "github.com/Noooste/fhttp"
	log "github.com/sirupsen/logrus"
	"golang.org/x/time/rate"

	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/flags"
	"github.com/sagan/erodownloader/util"
)

type FlaresolverrApiRequest struct {
	Cmd               string `json:"cmd,omitempty"`
	Url               string `json:"url,omitempty"`
	MaxTimeout        int    `json:"maxTimeout,omitempty"`
	ReturnOnlyCookies bool   `json:"returnOnlyCookies,omitempty"`
}

type FlaresolverrApiResponseCookie struct {
	Name    string
	Value   string
	Domain  string
	Path    string
	Expires int64
}

type FlaresolverrApiResponse struct {
	Status   string            `json:"status,omitempty"`  // "ok"
	Message  string            `json:"message,omitempty"` // "Challenge solved!"
	Solution *FlaresolverrData `json:"solution,omitempty"`
}

type FlaresolverrReq struct {
	Client       *azuretls.Session
	Req          *azuretls.Request
	Flaresolverr string
}

// "solution" field in flaresolverr API response
type FlaresolverrData struct {
	Url       string                           `json:"url,omitempty"`
	Headers   map[string]string                `json:"headers,omitempty"`
	Status    int                              `json:"status,omitempty"`
	UserAgent string                           `json:"userAgent,omitempty"`
	Response  string                           `json:"response,omitempty"`
	Cookies   []*FlaresolverrApiResponseCookie `json:"cookies,omitempty"`
}

var (
	defaultClient      *azuretls.Session
	localClient        *azuretls.Session                               // no proxy
	defaultRateLimiter = rate.NewLimiter(rate.Every(time.Second*3), 3) // 3 requests per 3 seconds
	strictRateLimiter  = rate.NewLimiter(rate.Every(time.Second*9), 3) // 3 requests per 9 seconds
	flareSolverrReqCh  = make(chan *FlaresolverrReq)
	flareSolverrDataCh = make(chan *FlaresolverrData)
)

var strictRateLimiterDomains = []string{
	"asmrconnecting.xyz",
}

// "Access denied", "Attention Required! | Cloudflare"
var CF_ACCESS_DENIED_TITLES = []string{
	"Access denied",
	"Attention Required! | Cloudflare",
}

// "Just a moment...", "DDoS-Guard"
var CF_CHALLENGE_TITLES = []string{
	"Just a moment...",
	"DDoS-Guard",
}

func IsLocalUrl(urlObj *url.URL) bool {
	return constants.PrivateIpRegexp.MatchString(urlObj.Hostname()) || urlObj.Hostname() == "localhost"
}

func getRateLimiter(req *azuretls.Request) *rate.Limiter {
	if req == nil {
		return nil
	}
	urlObj, err := url.Parse(req.Url)
	if err != nil || IsLocalUrl(urlObj) {
		return nil
	}
	if slices.Contains(strictRateLimiterDomains, urlObj.Hostname()) {
		return strictRateLimiter
	}
	return defaultRateLimiter
}

func Init() {
	defaultClient = azuretls.NewSession()
	localClient = azuretls.NewSession()
	defaultClient.InsecureSkipVerify = true
	localClient.InsecureSkipVerify = true
	if proxy := util.FirstNonZeroArg(flags.Proxy, os.Getenv("HTTPS_PROXY"),
		os.Getenv("https_proxy")); proxy != "" && proxy != constants.NONE {
		if err := defaultClient.SetProxy(proxy); err != nil {
			log.Fatalf("failed to set proxy to %q: %v", proxy, err)
		}
		log.Warnf("Set proxy to %q (does not apply to local addresses)", proxy)
	}
	loadConfig()
	defaultClient.PreHookWithContext = func(ctx *azuretls.Context) error {
		if rateLimiter := getRateLimiter(ctx.Request); rateLimiter != nil {
			rateLimiter.Wait(context.TODO())
		}
		return nil
	}
	go func(input <-chan *FlaresolverrReq, output chan *FlaresolverrData) {
		for {
			req := <-input
			payload := &FlaresolverrApiRequest{
				Cmd:               "request.get",
				Url:               req.Req.Url,
				MaxTimeout:        300000,
				ReturnOnlyCookies: true,
			}
			var resBody *FlaresolverrApiResponse
			err := PostAndFetchJson(req.Flaresolverr, payload, &resBody, false)
			if err != nil {
				log.Tracef("flaresolverr api request error: %v", err)
				output <- &FlaresolverrData{
					Status: 500,
				}
			} else if resBody.Status != "ok" {
				output <- &FlaresolverrData{
					Status: 503,
				}
			} else {
				output <- resBody.Solution
			}
		}
	}(flareSolverrReqCh, flareSolverrDataCh)
}

func loadConfig() {
	// session headers is broken !
	// if config.Data.UserAgent != "" {
	// 	defaultClient.OrderedHeaders.Set("User-Agent", config.Data.UserAgent)
	// }
	for _, cookie := range config.Data.Cookies {
		urlStr := "https://" + strings.TrimPrefix(cookie.Domain, ".") + "/"
		if urlObj, err := url.Parse(urlStr); err == nil {
			defaultClient.CookieJar.SetCookies(urlObj, []*fhttp.Cookie{cookie})
		}
	}
}

func flareSolverr(client *azuretls.Session, req *azuretls.Request, flaresolverr string) *FlaresolverrData {
	flareSolverrReqCh <- &FlaresolverrReq{Client: client, Req: req, Flaresolverr: flaresolverr}
	return <-flareSolverrDataCh
}

func HttpRequest(req *azuretls.Request, useFlareSolverr bool) (
	res *azuretls.Response, err error) {
	client := defaultClient
	if urlObj, err := url.Parse(req.Url); err == nil && IsLocalUrl(urlObj) {
		client = localClient
	}
	req.TimeOut = time.Second * 30000
	if config.Data.UserAgent != "" {
		req.OrderedHeaders.Set("User-Agent", config.Data.UserAgent)
	}
	util.LogAzureHttpRequest(req)
	res, err = client.Do(req)
	util.LogAzureHttpResponse(res, err)
	if err != nil {
		// workaround for a azuretls bug that request to a host always returns EOF after sending some requests.
		// the error may be EOF, or the below:
		// read tcp 192.168.1.1:12345->1.2.3.4:443: wsarecv:
		// A connection attempt failed because the connected party did not properly respond after a period of time,
		// or established connection failed because connected host has failed to respond.
		if errors.Is(err, io.EOF) ||
			strings.Contains(err.Error(), "connected party did not properly respond after a period of time") ||
			strings.Contains(err.Error(), "use of closed network connection") {
			CloseHost(req.Url)
		}
		return res, err
	}
	if res.StatusCode == 403 {
		for _, title := range CF_ACCESS_DENIED_TITLES {
			if strings.Contains(string(res.Body), "<title>"+title+"</title") {
				return res, fmt.Errorf("your ip is possibly blocked by cloudflare")
			}
		}
		challenge := false
		for _, title := range CF_CHALLENGE_TITLES {
			if strings.Contains(string(res.Body), "<title>"+title+"</title") {
				challenge = true
				break
			}
		}
		if !challenge {
			return res, err
		}
		if !useFlareSolverr || config.Data.FlareSolverr == "" || config.Test1 {
			return res, fmt.Errorf("request blocked by cloudflare challenge, setup flaresolverr to proceed")
		}
		log.Tracef("Detected cloudflare challenge, solving using %s", config.Data.FlareSolverr)
		data := flareSolverr(client, req, config.Data.FlareSolverr)
		if data.Status != 200 && data.Status != 0 { // If returnOnlyCookies is set, status is 0
			return res, fmt.Errorf("failed to resolve cloueflare challenge, status=%d", data.Status)
		}
		log.Tracef("flaresolverr solved: %v", data)
		cookies := util.Map(data.Cookies, func(c *FlaresolverrApiResponseCookie) *fhttp.Cookie {
			return &fhttp.Cookie{
				Name:   c.Name,
				Value:  c.Value,
				Path:   c.Path,
				Domain: c.Domain,
				MaxAge: 86400 * 365,
			}
		})
		config.UpdateCookies(data.UserAgent, cookies)
		loadConfig()
		return client.Do(&azuretls.Request{
			Url:            req.Url,
			Method:         req.Method,
			Body:           req.Body,
			OrderedHeaders: req.OrderedHeaders,
			TimeOut:        req.TimeOut,
		})
	}
	return res, err
}

// If addext is true, the ext of fileUrl will be appended to filename.
// The ext of fileUrl is guessed from response content-type and / or fileUrl path.
// Return created filename and ext of fileUrl.
func SaveUrl(fileUrl string, filename string, addext bool, useFlareSolverr bool) (
	createdfile string, ext string, err error) {
	res, err := HttpRequest(&azuretls.Request{
		Method:     http.MethodGet,
		Url:        fileUrl,
		IgnoreBody: true,
	}, useFlareSolverr)
	if err != nil {
		return "", "", fmt.Errorf("failed to fetch %q: %w", fileUrl, err)
	}
	defer res.CloseBody()
	if res.StatusCode != 200 {
		return "", "", fmt.Errorf("failed to fetch %q: status=%d", fileUrl, res.StatusCode)
	}
	ext = util.GetExtFromType(res.Header.Get("Content-Type"))
	if ext == "" {
		if urlObj, err := url.Parse(fileUrl); err == nil {
			ext = strings.ToLower(path.Ext(urlObj.Path))
		}
	}
	if addext {
		filename += ext
	}
	file, err := os.Create(filename)
	if err != nil {
		return "", "", fmt.Errorf("failed to open file: %w", err)
	}
	defer file.Close()
	_, err = io.Copy(file, res.RawBody)
	return filename, ext, err
}

func FetchUrl(url string, header http.Header, useFlareSolverr bool) (*azuretls.Response, error) {
	req := &azuretls.Request{
		Method: http.MethodGet,
		Url:    url,
	}
	for name := range header {
		req.OrderedHeaders.Set(name, header.Get(name))
	}
	res, err := HttpRequest(req, useFlareSolverr)
	if err != nil {
		return nil, fmt.Errorf("failed to fetch url: %w", err)
	}
	if res.StatusCode != 200 {
		return res, fmt.Errorf("failed to fetch url: status=%d", res.StatusCode)
	}
	return res, nil
}

func FetchJson(url string, v any, useFlareSolverr bool) error {
	res, err := FetchUrl(url, nil, useFlareSolverr)
	if err != nil {
		return err
	}
	if v != nil {
		err = json.Unmarshal(res.Body, v)
	}
	return err
}

func PostAndFetchJson(url string, reqBody any, resBody any, useFlareSolverr bool) (err error) {
	reqData, err := json.Marshal(reqBody)
	if err != nil {
		return fmt.Errorf("failed to marshal json: %w", err)
	}
	res, err := HttpRequest(&azuretls.Request{
		Method: http.MethodPost,
		Body:   reqData,
		OrderedHeaders: [][]string{
			{"Content-Type", "application/json"},
		},
		Url: url,
	}, useFlareSolverr)
	if err != nil {
		return err
	}
	if res.StatusCode != 200 {
		return fmt.Errorf("PostAndFetchJson response error: status=%d", res.StatusCode)
	}
	err = json.Unmarshal(res.Body, resBody)
	return err
}

func CloseHost(urlStr string) {
	urlObj, err := url.Parse(urlStr)
	if err != nil {
		return
	}
	log.Debugf("remove %s from connection pool", urlStr)
	defaultClient.Connections.Remove(urlObj)
	localClient.Connections.Remove(urlObj)
}
