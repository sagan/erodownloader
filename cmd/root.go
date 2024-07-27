package cmd

import (
	"os"
	"sync"

	log "github.com/sirupsen/logrus"
	"github.com/spf13/cobra"

	"github.com/sagan/erodownloader/config"
	"github.com/sagan/erodownloader/flags"
	"github.com/sagan/erodownloader/httpclient"
)

// rootCmd represents the base command when called without any subcommands
var RootCmd = &cobra.Command{
	Use:   "erodownloader",
	Short: "erodownloader is a command-line program which helps you download and organize ero.",
	Long: `erodownloader is a command-line program which helps you download and organize ero.
It's a free and open-source software released under the AGPL-3.0 license,
visit https://github.com/sagan/erodownloader for source codes and other infomation.`,
	SilenceUsage: true,
	// Uncomment the following line if your bare application
	// has an action associated with it:
	// Run: func(cmd *cobra.Command, args []string) { },
}

func Execute() {
	cobra.OnInitialize(sync.OnceFunc(func() {
		// level: panic(0), fatal(1), error(2), warn(3), info(4), debug(5), trace(6).
		log.SetLevel(log.Level(3 + config.VerboseLevel))
		if err := config.Load(); err != nil {
			log.Fatalf("Failed to load config: %v", err)
		}
		httpclient.Init()
	}))
	if err := RootCmd.Execute(); err != nil {
		os.Exit(1)
	}
}

func init() {
	RootCmd.PersistentFlags().BoolVarP(&flags.DumpHeaders, "dump-headers", "", false,
		`Dump HTTP headers to log (error level)`)
	RootCmd.PersistentFlags().BoolVarP(&flags.DumpBody, "dump-body", "", false,
		`Dump HTTP body to log (error level)`)
	RootCmd.PersistentFlags().BoolVarP(&config.Test1, "test1", "", false, "test flag1")
	RootCmd.PersistentFlags().BoolVarP(&config.Test2, "test2", "", false, "test flag2")
	RootCmd.PersistentFlags().StringVarP(&config.ConfigFile, "config", "", config.DefaultConfigFile, "Config file")
	RootCmd.PersistentFlags().StringVarP(&flags.Proxy, "proxy", "", "",
		`Set proxy. If not set, will try to get proxy from HTTPS_PROXY env. `+
			`E.g. "http://127.0.0.1:1080", "socks5://127.0.0.1:7890"`)
	RootCmd.PersistentFlags().CountVarP(&config.VerboseLevel, "verbose", "v", "verbose (-v, -vv, -vvv)")
}
