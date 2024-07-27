package decensorship

import (
	"bytes"
	"net/url"
	"path/filepath"
	"slices"

	"github.com/sagan/erodownloader/constants"
	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/util/stringutil"
	"github.com/saintfish/chardet"
)

var bomTextualExts = []string{}

// Convert input text contents to UTF-8 encoding and "\n" line breaks.
// If changed is false, the input is already a normalized string.
func textNormalize(input []byte, options url.Values, logger transform.Logger) (output []byte, changed bool, err error) {
	// If bom is true, the output will have UTF-8 BOM; Otherwise the BOM will be strpped.
	bom := options.Get("bom") == "1" || slices.Contains(bomTextualExts, filepath.Ext(options.Get("input")))
	output = input
	detector := chardet.NewTextDetector()
	charset, err := detector.DetectBest(input)
	if err != nil || charset.Confidence < 100 {
		logger("can not get text file encoding: %v, err=%v", charset, err)
		return
	}
	if charset.Charset != "UTF-8" {
		logger("detected %s encoding, convert to UTF-8", charset.Charset)
		force := charset.Confidence == 100 && len(input) >= 512
		if output, err = stringutil.DecodeText(output, charset.Charset, force); err != nil {
			logger("can not convert text encoding: %v", err)
			return
		}
		changed = true
	}
	crlf := []byte{'\r', '\n'}
	cr := []byte{'\r'}
	lf := []byte{'\n'}
	if bytes.Contains(output, crlf) {
		logger("replace crlf to lf")
		output = bytes.ReplaceAll(output, crlf, lf)
		changed = true
	}
	if bytes.Contains(output, cr) {
		logger("replace cr to lf")
		output = bytes.ReplaceAll(output, cr, lf)
		changed = true
	}
	hasBom := bytes.HasPrefix(output, constants.Utf8bom)
	if bom {
		if !hasBom {
			logger("add UTF-8 bom")
			output = append(output, constants.Utf8bom...)
			output = append(output, output...)
			changed = true
		}
	} else if hasBom {
		logger("remove UTF-8 bom")
		output = output[len(constants.Utf8bom):]
		changed = true
	}
	return
}
