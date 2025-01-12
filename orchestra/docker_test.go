package orchestra_test

import (
	"context"
	"strings"
	"testing"

	"github.com/google/uuid"
	"github.com/jtarchie/ci/orchestra"
	. "github.com/onsi/gomega"
)

func TestDockerRun(t *testing.T) {
	assert := NewGomegaWithT(t)
	client, err := orchestra.NewDocker("test")
	assert.Expect(err).NotTo(HaveOccurred())

	id, err := uuid.NewV7()
	assert.Expect(err).NotTo(HaveOccurred())

	container, err := client.RunContainer(
		context.Background(),
		orchestra.Task{
			ID:      id.String(),
			Image:   "alpine",
			Command: []string{"echo", "hello"},
		},
	)
	assert.Expect(err).NotTo(HaveOccurred())

	assert.Eventually(func() bool {
		status, err := container.Status(context.Background())
		assert.Expect(err).NotTo(HaveOccurred())

		return status.IsDone() && status.ExitCode() == 0
	}).Should(BeTrue())

	stdout, stderr := &strings.Builder{}, &strings.Builder{}
	assert.Eventually(func() bool {
		err := container.Logs(context.Background(), stdout, stderr)
		assert.Expect(err).NotTo(HaveOccurred())

		return strings.Contains(stdout.String(), "hello")
	}).Should(BeTrue())

	// running a container should be deterministic and idempotent
	container, err = client.RunContainer(
		context.Background(),
		orchestra.Task{
			ID:      id.String(),
			Image:   "alpine",
			Command: []string{"echo", "hello"},
		},
	)
	assert.Expect(err).NotTo(HaveOccurred())

	assert.Eventually(func() bool {
		status, err := container.Status(context.Background())
		assert.Expect(err).NotTo(HaveOccurred())

		return status.IsDone() && status.ExitCode() == 0
	}).Should(BeTrue())

	stdout, stderr = &strings.Builder{}, &strings.Builder{}
	assert.Eventually(func() bool {
		err := container.Logs(context.Background(), stdout, stderr)
		assert.Expect(err).NotTo(HaveOccurred())
		return strings.Contains(stdout.String(), "hello")
	}).Should(BeTrue())

	err = container.Cleanup(context.Background())
	assert.Expect(err).NotTo(HaveOccurred())
}
