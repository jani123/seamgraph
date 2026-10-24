package main

import (
	"seamgraph/internal/bootstrap"

	"github.com/spf13/cobra"
)

func newAgentCommand() *cobra.Command {
	agent := &cobra.Command{Use: "agent", Short: "Configure architecture guidance for coding agents"}
	var root string
	setup := &cobra.Command{
		Use: "setup", Short: "Install or update the AGENTS.md section and SeamGraph skill",
		Long: "Install or update a managed AGENTS.md section and .agents/skills/seamgraph/SKILL.md.\nPreserve instructions outside the managed section; reject malformed markers and unmanaged skills.\nUse Devbox commands when devbox.json or devbox.jsonc exists. Rerun after upgrading SeamGraph.",
		Args: cobra.NoArgs,
		RunE: func(cmd *cobra.Command, _ []string) error {
			svc, err := bootstrap.New(root)
			if err != nil {
				return err
			}
			changed, err := svc.SetupAgent(root)
			for _, path := range changed {
				cmd.Println("Updated", path)
			}
			if err != nil {
				return err
			}
			if len(changed) == 0 {
				cmd.Println("SeamGraph agent guidance is already up to date.")
			}
			return nil
		},
	}
	setup.Flags().StringVar(&root, "root", ".", "project root")
	agent.AddCommand(setup)
	return agent
}
