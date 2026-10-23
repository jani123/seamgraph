// Controlled Git execution preserves credentials while disabling hooks and unsafe transports.
package gitworkspace

import (
	"context"
	"os"
	"os/exec"
	"strings"
)

// Command preserves the user's SSH agent, known_hosts and Git credential setup,
// while removing environment variables that can redirect repository operations.
func Command(ctx context.Context, dir string, args ...string) *exec.Cmd {
	prefix := []string{"-c", "core.hooksPath=" + os.DevNull, "-c", "protocol.file.allow=never", "-c", "protocol.ext.allow=never"}
	if dir != "" {
		prefix = append(prefix, "-C", dir)
	}
	// #nosec G204 -- Controlled git argv execution disables hooks/unsafe transports and uses no shell.
	cmd := exec.CommandContext(ctx, "git", append(prefix, args...)...)
	for _, entry := range os.Environ() {
		key, _, _ := strings.Cut(entry, "=")
		if strings.HasPrefix(key, "GIT_") && key != "GIT_SSH_COMMAND" && key != "GIT_SSH" && key != "GIT_SSH_VARIANT" && key != "GIT_ASKPASS" && key != "GIT_CONFIG_GLOBAL" && key != "GIT_CONFIG_SYSTEM" && key != "GIT_CONFIG_NOSYSTEM" {
			continue
		}
		cmd.Env = append(cmd.Env, entry)
	}
	cmd.Env = append(cmd.Env, "GIT_TERMINAL_PROMPT=0")
	return cmd
}
