package util

import (
	"mime"
	"net/http"
	"slices"
	"strings"

	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/erodownloader/flags"
)

var textualMimes = []string{
	"application/json",
	"application/xml",
	"application/x-www-form-urlencoded",
	// "multipart/form-data",
}

// Log if dump-headers flag is set.
func LogAzureHttpRequest(req *azuretls.Request) {
	if flags.DumpHeaders || flags.DumpBodies {
		log.WithFields(log.Fields{
			"header": req.OrderedHeaders,
			"method": req.Method,
			"url":    req.Url,
		}).Errorf("http request")
	}
	if flags.DumpBodies {
		if body, ok := req.Body.([]byte); ok {
			logBody("http request body", http.Header(req.Header), body)
		} else {
			log.Errorf("http request body: %v", req.Body)
		}
	}
}

// Log if dump-headers flag is set.
func LogAzureHttpResponse(res *azuretls.Response, err error) {
	if flags.DumpHeaders || flags.DumpBodies {
		if res != nil {
			log.WithFields(log.Fields{
				"header": res.Header,
				"status": res.StatusCode,
				"error":  err,
			}).Errorf("http response")
		} else {
			log.WithFields(log.Fields{
				"error": err,
			}).Errorf("http response")
		}
	}
	if res != nil && flags.DumpBodies {
		logBody("http response body", http.Header(res.Header), res.Body)
	}
}

func getContentType(header http.Header) (contentType string, isText bool) {
	contentType, _, _ = mime.ParseMediaType(header.Get("Content-Type"))
	isText = slices.Contains(textualMimes, contentType) || strings.HasPrefix(contentType, "text/")
	return
}

func logBody(title string, header http.Header, body []byte) {
	maxBinaryBody := 1024
	contentType, isText := getContentType(header)
	if isText {
		log.WithFields(log.Fields{
			"body":        string(body),
			"contentType": contentType,
		}).Errorf(title)
	} else if len(body) <= maxBinaryBody {
		log.WithFields(log.Fields{
			"body":        body,
			"contentType": contentType,
		}).Errorf(title)
	} else {
		log.WithFields(log.Fields{
			"body_start":  body[:1024],
			"length":      len(body),
			"contentType": contentType,
		}).Errorf(title)
	}
}
