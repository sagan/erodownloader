package testtxt

import (
	"fmt"
	"os"

	"github.com/saintfish/chardet"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/util/helper"
	"github.com/sagan/erodownloader/util/stringutil"
)

var command = &cobra.Command{
	Use:   "testtxt {filename.txt}...",
	Short: "Test the file encoding of txt files.",
	Long:  `Test the file encoding of txt files.`,
	Args:  cobra.MatchAll(cobra.MinimumNArgs(1), cobra.OnlyValidArgs),
	RunE:  testtxt,
}

var (
	all bool
)

func init() {
	command.Flags().BoolVarP(&all, "all", "a", false, "Show encoding details of each file")
	cmd.RootCmd.AddCommand(command)
}

func testtxt(cmd *cobra.Command, args []string) (err error) {
	filenames := helper.ParseFilenameArgs(args...)

	errorCnt := 0
	detector := chardet.NewTextDetector()
	for _, filename := range filenames {
		data, err := os.ReadFile(filename)
		if err != nil {
			fmt.Printf("X %q: failed to read file: %v\n", filename, err)
			errorCnt++
			continue
		}
		charsets, err := detector.DetectAll(data)

		if err != nil || len(charsets) == 0 || charsets[0].Confidence != 100 {
			fmt.Printf("X %q: no determinitic encoding. possible encodings: %v, err=%v\n", filename, charsets, err)
			errorCnt++
		} else {
			fmt.Printf("%q: encoding: %s. Other possible encodings: %v, err=%v\n",
				filename, charsets[0].Charset, charsets, err)
		}
		if !all {
			continue
		}
		for _, charset := range charsets {
			output, err := stringutil.DecodeText(data, charset.Charset, true)
			if len(output) > 128 {
				output = output[:128]
			}
			if err != nil {
				fmt.Printf("%s: !invalid: %v\n", charset.Charset, err)
			} else {
				fmt.Printf("%s:\n:-----\n%s\n-----\n\n", charset.Charset, string(output))
			}
		}
		fmt.Printf("\n\n")
	}

	if errorCnt > 0 {
		return fmt.Errorf("%d errors", errorCnt)
	}
	return nil
}
