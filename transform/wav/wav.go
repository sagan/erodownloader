package wav

import (
	"strings"

	"github.com/sagan/erodownloader/transform"
	"github.com/sagan/erodownloader/transform/executor"
)

var ignore_flac_errors = []string{
	// unsupported format type: 3, 85 ...
	// 3: wav 是 float32 音频。flac 还不支持这种格式。
	// 如果用 "ffmpeg -i input.wav input.flac"，会将 wav 重编码为 24 bit flac，导致信息丢失。
	// https://unix.stackexchange.com/questions/772611/
	"unsupported format type",
	"unsupported WAVEFORMATEXTENSIBLE chunk",
	"has an ID3v2 tag",
	"--keep-foreign-metadata can only be used with WAVE",
	"ERROR: got partial sample",
	" is not a WAVE file",
	"ERROR: input file", // workaround
}

func init() {
	executorOptions := &executor.ExecutorOptions{
		Hardlink: true, // Workaround for https://github.com/xiph/flac/issues/713 .
		Binary:   "flac",
		BinaryArgs: []string{"--best", "--keep-foreign-metadata-if-present",
			"--output-name", executor.OUTPUT_PLACEHOLDER, executor.INPUT_PLACEHOLDER},
		OnError: func(combinedOutput []byte, ierr error, logger transform.Logger) (newArgs []string, err error) {
			output := string(combinedOutput)
			if strings.Contains(output, "read failed in WAVE/AIFF file (011)") {
				// 默认保留 wav 里的 foreign metadata 。部分 wav 文件里的 foreign metatada flac 报错无法读取，则不保留。
				// https://github.com/xiph/flac/blob/master/src/flac/foreign_metadata.c
				logger("Failed to read wav metadata, do not preserve it")
				return []string{"--best", "--output-name", executor.OUTPUT_PLACEHOLDER, executor.INPUT_PLACEHOLDER}, nil
			}
			for _, ignore_error := range ignore_flac_errors {
				if strings.Contains(output, ignore_error) {
					logger("wav can not be converted to flac: %s", ignore_error)
					return nil, nil
				}
			}
			logger("flac output: %s", output)
			return nil, ierr
		},
		Ext:    []string{".wav"},
		NewExt: ".flac",
		// Output: executor.BASE_PLACEHOLDER + ".flac",
		RenameAdditionalSuffixes: []string{".vtt", ".ass", ".srt", ".lrc"},
	}
	transform.Register(&transform.Transformer{
		Name:   "wav",
		Action: executorOptions.Transformer,
	})
}
