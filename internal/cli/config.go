package cli

import (
	"errors"
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
	"github.com/jmcampanini/cmdk/internal/safetext"
)

type configCommandOptions struct {
	showProvenance bool
	validatePath   string
}

func newConfigCommand(root *rootOptions) *cobra.Command {
	var options configCommandOptions

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show the effective configuration or validate a file",
		Long: `Show the configuration cmdk runs with, or validate a configuration file.

Stdout: the effective configuration as TOML after defaults and the
configuration file were applied, preceded by a comment naming the quoted
file path. Launch modes, window-name templates, picker delimiters, and icon
aliases are resolved before reporting.
A "# Derived values" section lists those values as TOML comments, so the
report loads back unchanged. --provenance appends a "# Provenance" table as
TOML comments naming the input that set each value. No values are redacted;
cmdk has no configuration fields designated as secrets.
--validate <path> loads that file instead of the discovered one, prints
"valid" on success, and otherwise exits 1 with the first problem on stderr.
Stderr: diagnostics. The command never prompts and needs no terminal.

` + configLoadingHelp + ` Unknown keys are rejected by name.

Configuration keys

` + config.RenderReference(),
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigCommand(root, cmd, options)
		},
	}
	cmd.Flags().BoolVar(&options.showProvenance, "provenance", false, "append the source of each value")
	cmd.Flags().StringVar(&options.validatePath, "validate", "", "validate this file and print \"valid\"")
	return cmd
}

func runConfigCommand(root *rootOptions, cmd *cobra.Command, options configCommandOptions) error {
	if cmd.Flags().Changed("validate") {
		if options.validatePath == "" {
			return errors.New("config path cannot be empty")
		}
		if err := config.ValidateFile(options.validatePath); err != nil {
			return err
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "valid")
		return err
	}

	loaded, err := root.loadConfig(cmd)
	if err != nil {
		return err
	}

	return writeConfigReport(cmd.OutOrStdout(), loaded, options.showProvenance)
}

func writeConfigReport(out io.Writer, loaded loadedConfig, showProvenance bool) error {
	reporter := configreporter.New(loaded.config, loaded.report)
	if _, err := fmt.Fprintf(out, "# %q\n", loaded.path); err != nil {
		return err
	}
	if err := reporter.WriteTOML(out); err != nil {
		return err
	}
	if err := loaded.config.WriteDerivedValues(out); err != nil {
		return err
	}
	if !showProvenance {
		return nil
	}

	if _, err := fmt.Fprintln(out, "\n# Provenance"); err != nil {
		return err
	}
	return writeProvenanceTable(out, reporter)
}

func writeProvenanceTable(out io.Writer, reporter configreporter.Reporter[config.Config]) error {
	headers := reporter.ProvenanceHeaders()
	rows := reporter.ProvenanceRows()
	for _, row := range rows {
		for i, cell := range row {
			row[i] = safetext.EscapeTerminalControls(cell)
		}
	}
	lastColumn := len(headers) - 1
	paddedCell := lipgloss.NewStyle().PaddingRight(2)
	plainCell := lipgloss.NewStyle()
	provenance := table.New().
		BorderTop(false).
		BorderBottom(false).
		BorderLeft(false).
		BorderRight(false).
		BorderHeader(false).
		BorderColumn(false).
		StyleFunc(func(_ int, col int) lipgloss.Style {
			if col < lastColumn {
				return paddedCell
			}
			return plainCell
		}).
		Headers(headers...).
		Rows(rows...)

	for _, line := range strings.Split(provenance.String(), "\n") {
		if _, err := fmt.Fprintf(out, "# %s\n", strings.TrimRight(line, " ")); err != nil {
			return err
		}
	}
	return nil
}
