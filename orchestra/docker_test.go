package orchestra_test

import (
	"context"
	"strings"
	"testing"
	"time"

	"github.com/google/uuid"
	"github.com/jtarchie/ci/orchestra"
	"github.com/stretchr/testify/assert"
)

func TestDockerRun(t *testing.T) {
	client, err := orchestra.NewDocker("test")
	assert.NoError(t, err)

	id, err := uuid.NewV7()
	assert.NoError(t, err)

	stdout, stderr := &strings.Builder{}, &strings.Builder{}

	container, err := client.RunContainer(
		context.Background(),
		orchestra.Task{
			ID:      id.String(),
			Image:   "alpine",
			Command: []string{"echo", "hello"},
		},
	)
	assert.NoError(t, err)
	assert.Eventually(t, func() bool {
		status, err := container.Status(context.Background())
		assert.NoError(t, err)

		return status.IsDone() && status.ExitCode() == 0
	}, 5*time.Second, 100*time.Millisecond)
	assert.Eventually(t, func() bool {
		err := container.Logs(context.Background(), stdout, stderr)
		return err == nil && strings.Contains(stdout.String(), "hello")
	}, 5*time.Second, 100*time.Millisecond)
}
