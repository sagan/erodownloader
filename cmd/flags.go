package cmd

import (
	"errors"
	"slices"

	"github.com/spf13/cobra"
)

// an enum of string type
type EnumFlag struct {
	DefaultOptionIndex int
	Description        string
	Options            [][2]string // array of [option, desc] pairs
}

type EnumValue struct {
	value       *string
	description string
	options     [][2]string
}

func (ev *EnumValue) String() string {
	return *ev.value
}

func (ev *EnumValue) Set(value string) error {
	if !slices.ContainsFunc(ev.options, func(option [2]string) bool {
		return option[0] == value
	}) {
		return errors.New("must be " + ev.tip())
	}
	*ev.value = value
	return nil
}

func (ev *EnumValue) Type() string {
	return "string"
}

func (ev *EnumValue) tip() string {
	str := "any of: "
	for i, option := range ev.options {
		if i > 0 {
			str += "|"
		}
		str += option[0]
	}
	return str
}

func (ev *EnumValue) cobraUsage() string {
	str := ev.description
	str += ". Any of: "
	for i, option := range ev.options {
		if i > 0 {
			str += " | "
		}
		str += option[0]
		if option[1] != "" {
			str += " (" + option[1] + ")"
		}
	}
	return str
}

func AddEnumFlagP(command *cobra.Command, value *string, name string, shorthand string, flag *EnumFlag) {
	var vv = &EnumValue{
		value:       value,
		description: flag.Description,
		options:     flag.Options,
	}
	*vv.value = flag.Options[flag.DefaultOptionIndex][0]
	command.Flags().VarP(vv, name, shorthand, vv.cobraUsage())
}
