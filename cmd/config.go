package cmd

import (
	"fmt"
	"io"
	"text/tabwriter"

	"github.com/jmcampanini/go-config-loader/configreporter"
	"github.com/spf13/cobra"

	"github.com/jmcampanini/cmdk/internal/config"
)

var configCmd = newConfigCommand()

func newConfigCommand() *cobra.Command {
	var showProvenance bool
	var validatePath string

	cmd := &cobra.Command{
		Use:   "config",
		Short: "Show resolved configuration and validate a config file",
		Long:  "Show resolved configuration after applying defaults, the config file, and CMDK_* environment variables. Exits non-zero if the config is invalid.\n\nSee \"cmdk docs\" for the configuration reference and \"cmdk icons\" for icon aliases.",
		Args:  cobra.NoArgs,
		RunE: func(cmd *cobra.Command, args []string) error {
			if validatePath != "" {
				if err := config.ValidateFile(validatePath); err != nil {
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

			out := cmd.OutOrStdout()
			reporter := configreporter.New(cfg, report)
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
		},
	}
	cmd.Flags().BoolVar(&showProvenance, "provenance", false, "include config provenance")
	cmd.Flags().StringVar(&validatePath, "validate", "", "validate a config file and exit")
	return cmd
}

func writeProvenanceTable(out io.Writer, reporter configreporter.Reporter[config.Config]) error {
	w := tabwriter.NewWriter(out, 0, 0, 2, ' ', 0)
	headers := reporter.ProvenanceHeaders()
	if _, err := fmt.Fprintf(w, "# %s\t%s\t%s\n", headers[0], headers[1], headers[2]); err != nil {
		return err
	}
	for _, row := range reporter.ProvenanceRows() {
		if _, err := fmt.Fprintf(w, "# %s\t%s\t%s\n", row[0], row[1], row[2]); err != nil {
			return err
		}
	}
	return w.Flush()
}

func init() {
	rootCmd.AddCommand(configCmd)
}
