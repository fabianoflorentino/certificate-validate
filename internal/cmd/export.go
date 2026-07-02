package cmd

import (
	"context"
	"fmt"
	"os"

	"github.com/fabianoflorentino/certificate-validate/internal/config"
	"github.com/fabianoflorentino/certificate-validate/internal/formatter"
	"github.com/spf13/cobra"
)

var (
	exportFormat string
	exportFile   string
)

var exportCmd = &cobra.Command{
	Use:   "export",
	Short: "Export certificate data to JSON or CSV",
	Long: `Run a certificate check against configured hosts and export the results.

Output formats: json (default), csv

Use --output-file to write to a file instead of stdout.`,
	RunE: func(cmd *cobra.Command, args []string) error {
		cfg, err := config.Load(cfgPath)
		if err != nil {
			return fmt.Errorf("load config: %w", err)
		}

		warnings, err := cfg.Validate()
		if err != nil {
			return fmt.Errorf("invalid config: %w", err)
		}
		for _, w := range warnings {
			fmt.Fprintf(os.Stderr, "WARNING: %s\n", w)
		}

		app, err := buildApp(cfg)
		if err != nil {
			return err
		}

		hosts := toCheckerHostsFromConfig(cfg.Hosts)

		ctx := cmd.Context()
		if ctx == nil {
			ctx = context.Background()
		}
		certs, errs := app.CheckAll(ctx, hosts, 10)
		for _, e := range errs {
			fmt.Fprintf(os.Stderr, "ERROR: %v\n", e)
		}

		var data []byte
		switch exportFormat {
		case "csv":
			data, err = formatter.FormatCSV(certs)
		default:
			data, err = formatter.FormatJSON(certs)
		}
		if err != nil {
			return fmt.Errorf("format export: %w", err)
		}

		if exportFile != "" {
			if err := os.WriteFile(exportFile, data, 0644); err != nil {
				return fmt.Errorf("write export file: %w", err)
			}
		} else {
			fmt.Println(string(data))
		}

		return nil
	},
}

func init() {
	exportCmd.Flags().StringVarP(&exportFormat, "format", "f", "json",
		"output format: json or csv")
	exportCmd.Flags().StringVarP(&exportFile, "output-file", "o", "",
		"write output to file instead of stdout")
	rootCmd.AddCommand(exportCmd)
}
