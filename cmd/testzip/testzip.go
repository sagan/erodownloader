package testzip

import (
	"fmt"

	"github.com/saintfish/chardet"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/transform/decompress"
	"github.com/sagan/erodownloader/util/helper"
	"github.com/sagan/erodownloader/util/stringutil"
	"github.com/sagan/zip"
)

var command = &cobra.Command{
	Use:   "testzip {filename.zip}...",
	Short: "Test the filename encodings of zip files",
	Long:  `Test the filename encodings of zip files.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:  testzip,
}

var (
	all     bool
	zipmode int
)

func init() {
	command.Flags().BoolVarP(&all, "all", "a", false, "Show filename encoding details of each zip")
	command.Flags().IntVarP(&zipmode, "zipmode", "", config.DEFAULT_ZIPMODE,
		"Zip filename encoding detection mode. 0 -  strict; 1 - guess the best one (shift_jis > gbk)")
	cmd.RootCmd.AddCommand(command)
}

func testzip(cmd *cobra.Command, args []string) (err error) {
	filenames := helper.ParseFilenameArgs(args...)

	errorCnt := 0
	for _, filename := range filenames {
		zipFile, err := zip.OpenReader(filename)
		if err != nil {
			if err == zip.ErrInsecurePath {
				zipFile.Close()
			}
			fmt.Printf("%q: failed to open: %v", filename, err)
			errorCnt++
			continue
		}
		defer zipFile.Close()

		var rawFilenames []string
		for _, file := range zipFile.File {
			if file.NonUTF8 {
				rawFilenames = append(rawFilenames, file.Name)
			}
		}

		var encoding string
		var possibleEncodings []string
		if len(rawFilenames) == 0 {
			fmt.Printf("%q: all content filenames are ! marked as UTF-8\n", filename)
			fmt.Printf("\n")
		} else {
			encoding, possibleEncodings, err = decompress.DetectFilenamesEncoding(rawFilenames, zipmode)
			if err != nil {
				fmt.Printf("%q: failed to detect filenames encoding: possibilities=%v, err=%v\n",
					filename, possibleEncodings, err)
				errorCnt++
			} else {
				fmt.Printf("%q: detected filenames encoding: %s\n", filename, encoding)
			}
		}

		if !all {
			continue
		}
		fmt.Printf("\n")
		detector := chardet.NewTextDetector()
		for i, file := range zipFile.File {
			if !file.NonUTF8 {
				fmt.Printf("%-15d  %-10s  %v\n", i, "// UTF-8", file.Name)
				continue
			} else if stringutil.IsASCIIIndexBy8s32(file.Name) {
				fmt.Printf("%-15d  %-10s  %v\n", i, "// ASCII", file.Name)
				continue
			}
			results, err := detector.DetectAll([]byte(file.Name))
			if err != nil {
				fmt.Printf("%-15d  %-10s  %v\n", i, "ErrDetect", err)
				continue
			} else {
				fmt.Printf("%-15d  %-10s  %v\n", i, "Result", results)
			}
			fmt.Printf("%-15s  %-10s  %s\n", "UTF-8", "String", file.Name)
			for _, result := range results {
				if str, err := stringutil.DecodeText([]byte(file.Name), result.Charset, false); err != nil {
					fmt.Printf("%-15s  %-10s  %v\n", result.Charset, "Err", err)
				} else {
					fmt.Printf("%-15s  %-10s  %s\n", result.Charset, "String", str)
				}
			}
		}
		fmt.Printf("\n")

		// zip comment
		if zipFile.Comment != "" {
			if stringutil.IsASCIIIndexBy8s32(zipFile.Comment) {
				fmt.Printf("%-15s  %-10s  %s\n", "Comment", "ASCII", zipFile.Comment)
			} else {
				results, err := detector.DetectAll([]byte(zipFile.Comment))
				if err != nil {
					fmt.Printf("%-15s  %-10s  %v\n", "Comment", "ErrDetect", err)
					continue
				} else {
					fmt.Printf("%-15s  %-10s  %v\n", "Comment", "Result", results)
				}
				fmt.Printf("%-15s  %-10s  %s\n", "UTF-8", "String", zipFile.Comment)
				for _, result := range results {
					if str, err := stringutil.DecodeText([]byte(zipFile.Comment), result.Charset, false); err != nil {
						fmt.Printf("%-15s  %-10s  %v\n", result.Charset, "Err", err)
					} else {
						fmt.Printf("%-15s  %-10s  %s\n", result.Charset, "String", str)
					}
				}
			}
			fmt.Printf("\n")
		}
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
