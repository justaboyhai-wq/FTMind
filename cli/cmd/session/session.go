// Package sessioncmd holds `fmind session` command tree (list / view /
// delete / ask / resume / stop) for chat history and agent invocation.
//
// Package name `sessioncmd` (not `session`) so callers can `import sdk
// "github.com/justaboyhai-wq/fmind/client"` and use `sdk.Session` without
// shadowing - same hygiene as `profilecmd`.
package sessioncmd

import (
	"github.com/spf13/cobra"

	"github.com/justaboyhai-wq/fmind/cli/internal/cmdutil"
)

// NewCmd builds the `fmind session` parent command.
func NewCmd(f *cmdutil.Factory) *cobra.Command {
	cmd := &cobra.Command{
		Use:   "session",
		Short: "Manage chat sessions",
	}
	cmd.AddCommand(NewCmdList(f))
	cmd.AddCommand(NewCmdView(f))
	cmd.AddCommand(NewCmdDelete(f))
	cmd.AddCommand(NewCmdAsk(f))
	cmd.AddCommand(NewCmdResume(f))
	cmd.AddCommand(NewCmdStop(f))
	cmd.AddCommand(NewCmdToolApproval(f))
	return cmd
}
