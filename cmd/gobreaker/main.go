package main

import (
	"fmt"
	"os"
	"runtime/debug"

	"github.com/flaticols/gobreaker/internal/git"
	"github.com/jessevdk/go-flags"
)

type programOptions struct {
	//nolint:golines
	RepoPath string `short:"r" long:"repo" description:"Path to git repository (default: current directory)"`
	//nolint:golines
	OldRef string `short:"o" long:"old" description:"Old reference (branch, tag, or commit) to compare from, or 'latest' to compare latest against HEAD" required:"true"`
	NewRef string `short:"n" long:"new" description:"New reference (branch, tag, or commit) to compare to" default:"HEAD"`
	//nolint:golines
	Output          string `short:"f" long:"format" description:"Output format (text, json, markdown)" default:"text" choice:"text"`
	IncludeInternal bool   `short:"i" long:"include-internal" description:"Include internal packages in API analysis"`
	Quite           bool   `short:"q" long:"quiet" description:"Suppress output"`
	Version         bool   `short:"v" long:"version" description:"Print version information and exit"`
}

func main() {
	programCfg := programOptions{}
	p := flags.NewParser(&programCfg, flags.Default)

	args, err := p.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			os.Exit(0)
		}
		os.Exit(1)
	}

	if programCfg.Version {
		printVersion()
		os.Exit(0)
	}

	if programCfg.RepoPath == "" {
		wd, err := os.Getwd()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
		programCfg.RepoPath = wd
	} else {
		err := os.Chdir(programCfg.RepoPath)
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	if len(args) > 0 {
		_, _ = fmt.Fprintf(os.Stderr, "Error: unexpected arguments: %v\n", args)
		os.Exit(1)
	}

	diff, err := git.OpenRepo(programCfg.RepoPath, programCfg.OldRef, programCfg.NewRef, programCfg.IncludeInternal)
	if err != nil {
		_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
		os.Exit(1)
	}

	if !programCfg.Quite {
		err = diff.Reports()
		if err != nil {
			_, _ = fmt.Fprintf(os.Stderr, "Error: %v\n", err)
			os.Exit(1)
		}
	}

	if !diff.IsCompatible() {
		os.Exit(1)
	}

	os.Exit(0)
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
