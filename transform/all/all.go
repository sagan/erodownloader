package all

import (
	_ "github.com/sagan/erodownloader/transform"
	_ "github.com/sagan/erodownloader/transform/clean"
	_ "github.com/sagan/erodownloader/transform/correctext"
	_ "github.com/sagan/erodownloader/transform/decensorship"
	_ "github.com/sagan/erodownloader/transform/decompress"
	_ "github.com/sagan/erodownloader/transform/denesting"
	_ "github.com/sagan/erodownloader/transform/executor"
	_ "github.com/sagan/erodownloader/transform/nocredit"
	_ "github.com/sagan/erodownloader/transform/noempty"
	_ "github.com/sagan/erodownloader/transform/normalizename"
	_ "github.com/sagan/erodownloader/transform/text"
	_ "github.com/sagan/erodownloader/transform/wav"
)
