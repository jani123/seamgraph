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

// Execute runs a fixed runner and returns its actual artifact. The final error
// reports artifact/protocol failures; runErr reports a failing test execution.
func Execute(ctx context.Context, root, artifactDir, runner string) (output []byte, format string, runErr error, err error) {
	cmd, format, artifact, err := Command(ctx, root, artifactDir, runner)
	if err != nil {
		return nil, "", nil, err
	}
	if artifact != "" {
		if err = os.Remove(artifact); err != nil && !os.IsNotExist(err) {
			return nil, format, nil, err
		}
	}
	output, runErr = cmd.CombinedOutput()
	if artifact != "" {
		output, err = os.ReadFile(artifact)
		if err != nil {
			err = fmt.Errorf("runner did not produce an artifact: %w (runner: %v)", err, runErr)
		}
	}
	return
}
