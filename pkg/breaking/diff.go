package breaking

import (
	"fmt"
	"io"
	"os"

	"golang.org/x/exp/apidiff"
)

type DiffReport = map[string]apidiff.Report

// Diff represents the API differences between two versions of Go packages.
// It tracks both regular package changes and import changes, determining
// overall compatibility based on the presence of breaking changes.
type Diff struct {
	apiChangeReports DiffReport
	importsChange    DiffReport
	breakingImports  bool
	incompatible     bool
}

func New(apiCHanges, importCHanges DiffReport) *Diff {
	return &Diff{
		apiChangeReports: apiCHanges,
		importsChange:    importCHanges,
	}
}

func (d *Diff) SetBreakingImports(b bool) {
	d.breakingImports = b
}

func (d *Diff) SetIncompatible(b bool) {
	d.incompatible = b
}

// IsCompatible returns true if there are no breaking changes in either
// the package APIs or imports.
func (d *Diff) IsCompatible() bool {
	return !d.breakingImports && !d.incompatible
}

// Reports writes all API diff reports to stdout with proper formatting.
// It includes both package reports and import reports, showing both
// compatible and incompatible changes.
func (d *Diff) Reports() error {
	w := &TextDiffReport{prefix: "  ", w: os.Stdout}
	for pkg, report := range d.apiChangeReports {
		err := writeReport(w, pkg, report)
		if err != nil {
			return err
		}
	}

	for pkg, report := range d.importsChange {
		err := writeReport(w, pkg, report)
		if err != nil {
			return err
		}
	}

	return nil
}

func writeReport(w io.Writer, name string, report apidiff.Report) error {
	var (
		hasIncompatible bool
		hasCompatible   bool
	)

	for _, c := range report.Changes {
		if !c.Compatible {
			hasIncompatible = true
		} else {
			hasCompatible = true
		}
	}

	if hasIncompatible {
		if _, err := fmt.Fprintf(os.Stdout, "\n%s\n", name); err != nil {
			return err
		}

		if err := report.TextIncompatible(w, true); err != nil {
			return err
		}
	}

	if hasCompatible {
		if _, err := fmt.Fprintf(os.Stdout, "\n%s\n", name); err != nil {
			return err
		}
		if err := report.TextCompatible(w); err != nil {
			return err
		}
	}
	return nil
}
