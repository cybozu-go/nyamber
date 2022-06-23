package cmd

import (
	"context"

	"github.com/cybozu-go/well"
	"github.com/spf13/cobra"
)

var rootCmd = &cobra.Command{
	Use:   "entrypoint",
	Short: "DC test pod entrypoint",
	Long:  "DC test pod entrypoint",
	RunE: func(cmd *cobra.Command, args []string) error {
		_ = well.NewEnvironment(context.Background())
		// run neco bootstrap
		// run neco-apps bootstrap
		// serve execution status of bootstrap process (status endpoint)
		return nil
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {

}
