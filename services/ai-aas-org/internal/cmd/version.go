package cmd

import (
	"encoding/json"
	"fmt"
	"runtime"

	"github.com/spf13/cobra"

	"github.com/otherjamesbrown/ai-aas/services/cli-shared/output"
)

// versionCmd represents the version command
var versionCmd = &cobra.Command{
	Use:   "version",
	Short: "Show version information",
	Long:  `Display version, build, and runtime information for the CLI.`,
	Run:   runVersion,
}

func init() {
	rootCmd.AddCommand(versionCmd)
}

func runVersion(cmd *cobra.Command, args []string) {
	if IsJSONOutput() {
		printVersionJSON()
		return
	}

	printVersionText()
}

func printVersionText() {
	output.Header("ai-aas-org CLI")
	output.KeyValue("Version", versionInfo.Version)
	output.KeyValue("Commit", versionInfo.Commit)
	output.KeyValue("Built", versionInfo.BuildTime)
	output.KeyValue("Go Version", runtime.Version())
	output.KeyValue("OS/Arch", fmt.Sprintf("%s/%s", runtime.GOOS, runtime.GOARCH))
}

func printVersionJSON() {
	info := map[string]string{
		"version":    versionInfo.Version,
		"commit":     versionInfo.Commit,
		"build_time": versionInfo.BuildTime,
		"go_version": runtime.Version(),
		"os":         runtime.GOOS,
		"arch":       runtime.GOARCH,
	}
	data, _ := json.MarshalIndent(info, "", "  ")
	fmt.Println(string(data))
}
