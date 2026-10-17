package testresults

import (
	"context"
	"fmt"
	"os"
	"os/exec"
	"path/filepath"
)

// Command is the runner adapter boundary. Only these fixed invocations execute;
// project/YAML strings cannot become arbitrary shell commands.
func Command(ctx context.Context, root, artifactDir, runner string) (*exec.Cmd, string, string, error) {
	var cmd *exec.Cmd
	format, artifact := "", ""
	switch runner {
	case "go":
		cmd = exec.CommandContext(ctx, "go", "test", "-count=1", "-json", "./...")
		cmd.Env = append(os.Environ(), "GOPROXY=off", "GOSUMDB=off", "GOTOOLCHAIN=local")
		format = "go-test-json"
	case "pytest":
		artifact = filepath.Join(artifactDir, "pytest.xml")
		cmd = exec.CommandContext(ctx, "python3", "-m", "pytest", "--junitxml="+artifact)
		format = "junit"
	case "vitest":
		artifact = filepath.Join(artifactDir, "vitest.json")
		cmd = exec.CommandContext(ctx, "npm", "exec", "--offline", "--", "vitest", "run", "--reporter=json", "--outputFile="+artifact)
		format = "vitest-json"
	default:
		return nil, "", "", fmt.Errorf("supported runners: go, pytest, vitest")
	}
	cmd.Dir = root
	return cmd, format, artifact, nil
}
