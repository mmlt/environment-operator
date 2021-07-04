package cmd

import (
	"github.com/spf13/cobra"
)

// NewRootCommand returns the root of all envop commands.
func NewRootCommand() *cobra.Command {
	command := &cobra.Command{
		Use:   "envop",
		Short: "Environment operator",
		Long: `Environment operator (envop)
For normal operations run envop as Kubernetes controller with:
    envop controller
and then apply environment resources to the controller:
    envop apply

For testing purposes the controller can be run without making modifications:
    envop dryruncontroller
`,
		Run: func(cmd *cobra.Command, args []string) {
			cmd.HelpFunc()(cmd, args)
		},
		Hidden: true,
	}

	command.AddCommand(NewCmdController())
	command.AddCommand(NewDryrunControllerCmd())
	command.AddCommand(NewCmdApply())
	command.AddCommand(NewCmdReset())

	return command
}
