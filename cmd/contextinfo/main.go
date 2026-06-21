// Command contextinfo prints the detected run context (CI, git, runtime) in a
// choice of formats (json, json-flat, text, tfvars, tfvars-json).
package main

import (
	"encoding/json"
	"flag"
	"fmt"
	"os"

	"github.com/Th0masL/contextinfo/pkg/contextinfo"
)

// version is overridden at build time via -ldflags "-X main.version=...".
var version = "dev"

func main() {
	format := flag.String("format", "json", "output format: json, json-flat, text, tfvars, or tfvars-json")
	prefix := flag.String("prefix", "", "prefix for flattened keys (applies to json-flat, tfvars, tfvars-json)")
	showVersion := flag.Bool("version", false, "print version and exit")
	flag.Parse()

	if *showVersion {
		fmt.Println(version)
		return
	}

	info := contextinfo.Detect()

	switch *format {
	case "json":
		enc := json.NewEncoder(os.Stdout)
		enc.SetIndent("", "  ")
		_ = enc.Encode(info)
	case "json-flat":
		b, err := info.FlatJSON(*prefix)
		emit(b, err)
	case "text":
		printText(info)
	case "tfvars":
		fmt.Print(info.TFVarsHCL(*prefix))
	case "tfvars-json":
		b, err := info.TFVarsJSON(*prefix)
		emit(b, err)
	default:
		fmt.Fprintf(os.Stderr, "contextinfo: unknown format %q (want json, json-flat, text, tfvars, or tfvars-json)\n", *format)
		os.Exit(2)
	}
}

// emit writes b to stdout, or exits non-zero on a rendering error.
func emit(b []byte, err error) {
	if err != nil {
		fmt.Fprintf(os.Stderr, "contextinfo: %v\n", err)
		os.Exit(1)
	}
	os.Stdout.Write(b)
}

func printText(info contextinfo.Info) {
	rows := [][2]string{
		{"ci.detected", fmt.Sprintf("%t", info.CI.Detected)},
		{"ci.name", info.CI.Name},
		{"ci.build_url", info.CI.BuildURL},
		{"ci.build_number", info.CI.BuildNumber},
		{"git.commit", info.Git.Commit},
		{"git.branch", info.Git.Branch},
		{"git.tag", info.Git.Tag},
		{"git.dirty", fmt.Sprintf("%t", info.Git.Dirty)},
		{"git.remote", info.Git.Remote},
		{"runtime.os", info.Runtime.OS},
		{"runtime.arch", info.Runtime.Arch},
		{"runtime.hostname", info.Runtime.Hostname},
	}
	for _, r := range rows {
		fmt.Printf("%-17s %s\n", r[0], r[1])
	}
}
