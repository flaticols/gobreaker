package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/flaticols/gobreaker"
	"github.com/jessevdk/go-flags"
)

type programOptions struct {
	Path            bool   `short:"p" long:"path" description:"Interpret arguments as filesystem paths"`
	IncludeInternal bool   `short:"i" long:"include-internal" description:"Include internal packages in API analysis"`
	RepoPath        string `short:"r" long:"repo" description:"Path to git repository (default: current directory)"`
	Output          string `short:"f" long:"format" description:"Output format" default:"text" choice:"text"`
	Quiet           bool   `short:"q" long:"quiet" description:"Suppress output"`
	Version         bool   `short:"v" long:"version" description:"Print version information and exit"`
	Args            struct {
		A string `positional-arg-name:"base"`
		B string `positional-arg-name:"target"`
	} `positional-args:"yes"`
}

func main() {
	programCfg := programOptions{}
	p := flags.NewParser(&programCfg, flags.Default)

	if _, err := p.Parse(); err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if programCfg.Version {
		printVersion()
		os.Exit(0)
	}

	var (
		report *gobreaker.Report
		err    error
	)

	if programCfg.Path {
		if programCfg.Args.A == "" {
			_, _ = fmt.Fprintf(os.Stderr, "Error: base path argument is required in --path mode\n")
			os.Exit(1)
		}
		newPath := programCfg.Args.B
		if newPath == "" {
			newPath, err = os.Getwd()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}
		report, err = gobreaker.CompareFilesystems(programCfg.Args.A, newPath, programCfg.IncludeInternal)
	} else {
		repoPath := programCfg.RepoPath
		if repoPath == "" {
			repoPath, err = os.Getwd()
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
		}

		oldRef := programCfg.Args.A
		if oldRef == "" {
			oldRef, err = gobreaker.DetectDefaultBranch(repoPath)
			if err != nil {
				_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
				os.Exit(1)
			}
			fmt.Printf("Comparing against default branch: %s\n", oldRef)
		}

		report, err = gobreaker.CompareRefs(repoPath, oldRef, programCfg.Args.B, programCfg.IncludeInternal)
	}

	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !programCfg.Quiet {
		if err := report.WriteText(os.Stdout); err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	if !report.IsCompatible() {
		os.Exit(1)
	}
}

func printVersion() {
	if info, ok := debug.ReadBuildInfo(); ok {
		fmt.Printf("gobreaker version %s\n", info.Main.Version)
		fmt.Printf("go version %s\n", info.GoVersion)
		for _, dep := range info.Deps {
			if dep.Path == "golang.org/x/exp" {
				fmt.Printf("apidiff version %s\n", dep.Version)
				break
			}
		}
	} else {
		fmt.Println("gobreaker version (devel)")
	}
}
