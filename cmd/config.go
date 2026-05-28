package cmd

import (
	"fmt"
	"io"
	"strings"

	"charm.land/lipgloss/v2"
	"charm.land/lipgloss/v2/table"
	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
)

var configCmd = newConfigCommand()

type configCommandOptions struct {
	showProvenance bool
	validatePath   string
}

func newConfigCommand() *cobra.Command {
	var options configCommandOptions

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show resolved configuration and validate a config file",
		Long:  "Show resolved configuration after applying defaults and the config file. Exits non-zero if the config is invalid.\n\nSee \"cmdk docs\" for the configuration reference and \"cmdk icons\" for icon aliases.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfigCommand(cmd, options)
		},
	}
	cmd.Flags().BoolVar(&options.showProvenance, "provenance", false, "include config provenance")
	cmd.Flags().StringVar(&options.validatePath, "validate", "", "validate a config file and exit")
	return cmd
}

func runConfigCommand(cmd *cobra.Command, options configCommandOptions) error {
	if options.validatePath != "" {
		if err := config.ValidateFile(options.validatePath); err != nil {
			return err
		}
		_, err := fmt.Fprintln(cmd.OutOrStdout(), "valid")
		return err
	}

	path, err := resolveConfigPath()
	if err != nil {
		return err
	}
	cfg, report, err := config.LoadWithReport(path)
	if err != nil {
		return err
	}

	reporter := configreporter.New(cfg, report)
	return writeConfigReport(cmd.OutOrStdout(), path, reporter, options.showProvenance)
}

func writeConfigReport(out io.Writer, path string, reporter configreporter.Reporter[config.Config], showProvenance bool) error {
	if _, err := fmt.Fprintf(out, "# %s\n", path); err != nil {
		return err
	}
	if err := reporter.WriteTOML(out); err != nil {
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
			if col < len(headers)-1 {
				return paddedCell
			}
			return plainCell
		}).
		Headers(headers...).
		Rows(reporter.ProvenanceRows()...)

	for _, line := range strings.Split(provenance.String(), "\n") {
		if _, err := fmt.Fprintf(out, "# %s\n", strings.TrimRight(line, " ")); err != nil {
			return err
		}
	}
	return nil
}

func init() {
	rootCmd.AddCommand(configCmd)
}
