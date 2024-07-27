package common

import (
	"github.com/sagan/erodownloader/cmd"
	"github.com/sagan/erodownloader/constants"
)

var ResourceSortFlag = &cmd.EnumFlag{
	Description: "Sort field of resource",
	Options: [][2]string{
		{constants.NONE, ""},
		{"title", ""},
		{"author", ""},
		{"size", ""},
	},
}

// asc|desc
var OrderFlag = &cmd.EnumFlag{
	Description: "Sort order",
	Options: [][2]string{
		{"asc", ""},
		{"desc", ""},
	},
}
