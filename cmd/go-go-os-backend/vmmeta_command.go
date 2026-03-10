package main

import (
	"context"

	"github.com/go-go-golems/go-go-os-backend/pkg/vmmeta"
	"github.com/spf13/cobra"
)

func newVMMetaCommand() *cobra.Command {
	cmd := &cobra.Command{
		Use:   "vmmeta",
		Short: "Generate VM metadata and docs artifacts from authored card sources",
	}

	cmd.AddCommand(newVMMetaGenerateCommand())
	return cmd
}

func newVMMetaGenerateCommand() *cobra.Command {
	var opts vmmeta.GenerateOptions

	cmd := &cobra.Command{
		Use:   "generate",
		Short: "Scan card/docs sources and emit generated VM metadata artifacts",
		RunE: func(cmd *cobra.Command, _ []string) error {
			return vmmeta.GenerateAndWrite(context.Background(), opts)
		},
	}

	cmd.Flags().StringVar(&opts.PackID, "pack-id", "", "Runtime pack id to validate against")
	cmd.Flags().StringVar(&opts.CardsDir, "cards-dir", "", "Directory containing per-card VM source files")
	cmd.Flags().StringVar(&opts.DocsDir, "docs-dir", "", "Directory containing pack-level docs VM source files")
	cmd.Flags().StringVar(&opts.OutputJSON, "output-json", "", "Path to the generated JSON artifact")
	cmd.Flags().StringVar(&opts.OutputTS, "output-ts", "", "Path to the generated TypeScript artifact")

	_ = cmd.MarkFlagRequired("pack-id")
	_ = cmd.MarkFlagRequired("cards-dir")
	_ = cmd.MarkFlagRequired("output-json")
	_ = cmd.MarkFlagRequired("output-ts")

	return cmd
}
