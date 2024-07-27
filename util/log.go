package util

import (
	"github.com/Noooste/azuretls-client"
	log "github.com/sirupsen/logrus"

	"github.com/sagan/erodownloader/flags"
)

// Log if dump-headers flag is set.
func LogAzureHttpRequest(req *azuretls.Request) {
	if flags.DumpHeaders {
		log.WithFields(log.Fields{
			"header": req.OrderedHeaders,
			"method": req.Method,
			"url":    req.Url,
		}).Errorf("http request")
	}
	if flags.DumpBody {
		if body, ok := req.Body.([]byte); ok {
			log.Errorf("http request body: %s", body)
		} else {
			log.Errorf("http request body: %v", req.Body)
		}
	}
}

// Log if dump-headers flag is set.
func LogAzureHttpResponse(res *azuretls.Response, err error) {
	if flags.DumpHeaders {
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
	if res != nil && flags.DumpBody {
		log.Errorf("http response body: %s", res.Body)
	}
}
