package cmd

import (
	"errors"
	"fmt"
	"regexp"
	"strings"

	"github.com/cybozu-go/nyamber/pkg/constants"
	"github.com/cybozu-go/nyamber/pkg/entrypoint"
	"github.com/cybozu-go/well"
	"github.com/go-logr/logr"
	"github.com/go-logr/zapr"
	"github.com/spf13/cobra"
	"go.uber.org/zap"
)

var listenAddr string
var log logr.Logger
var reJobName = regexp.MustCompile("^[a-zA-Z][-_a-zA-Z0-9]*$")

var rootCmd = &cobra.Command{
	Use:          "entrypoint <JOB_NAME:COMMAND>...",
	Short:        "DC test pod entrypoint",
	Long:         "DC test pod entrypoint",
	Args:         cobra.MinimumNArgs(1),
	SilenceUsage: true,
	RunE: func(cmd *cobra.Command, args []string) error {
		jobs := make([]entrypoint.Job, 0, len(args))
		for _, job := range args {
			split := strings.Split(job, ":")
			if len(split) != 2 {
				return errors.New("wrong job format")
			}
			jobName := split[0]
			if !reJobName.MatchString(jobName) {
				return errors.New("unexpected characters in JOB_NAME")
			}

			command := split[1]
			if len(command) < 1 {
				return errors.New("COMMAND is empty")
			}
			splittedCmd := strings.Split(command, " ")

			jobs = append(jobs, entrypoint.Job{
				Name:    jobName,
				Command: splittedCmd[0],
				Args:    splittedCmd[1:],
			})
		}

		runner := entrypoint.Runner{
			ListenAddr: listenAddr,
			Logger:     log,
			Jobs:       jobs,
		}
		well.Go(runner.Run)
		well.Stop()
		return well.Wait()
	},
}

func Execute() error {
	return rootCmd.Execute()
}

func init() {
	fs := rootCmd.Flags()
	fs.StringVar(&listenAddr, "listen-address", fmt.Sprintf(":%d", constants.ListenPort), "Listening address and port.")
	zapLog, err := zap.NewDevelopment()
	if err != nil {
		panic(fmt.Sprintf("who watches the watchmen (%v)?", err))
	}
	log = zapr.NewLogger(zapLog)
}
