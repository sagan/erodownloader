package main

import (
	"os"
	"runtime"
	_ "time/tzdata"

	_ "github.com/sagan/erodownloader/client/all"
	"github.com/sagan/erodownloader/cmd"
	_ "github.com/sagan/erodownloader/cmd/all"
	_ "github.com/sagan/erodownloader/scraper/all"
	_ "github.com/sagan/erodownloader/site/all"
	_ "github.com/sagan/erodownloader/transform/all"
	_ "github.com/sagan/erodownloader/util/osutil"
)

func main() {
	if runtime.GOOS == "windows" {
		// https://github.com/golang/go/issues/43947
		os.Setenv("NoDefaultCurrentDirectoryInExePath", "1")
	}
	cmd.Execute()
}
