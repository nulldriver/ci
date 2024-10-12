package orchestra

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/pkg/stdcopy"
)

type Docker struct {
	client    *client.Client
	namespace string
}

type DockerContainer struct {
	id     string
	client *client.Client
	task   Task
}

type DockerContainerStatus struct {
	state *types.ContainerState
}

func NewDocker(namespace string) (*Docker, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Docker{
		client:    cli,
		namespace: namespace,
	}, nil
}

func (d *Docker) RunContainer(ctx context.Context, task Task) (*DockerContainer, error) {
	reader, err := d.client.ImagePull(ctx, task.Image, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to initiate pull image: %w", err)
	}

	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	response, err := d.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: task.Image,
			Cmd:   task.Command,
		},
		nil, nil, nil,
		fmt.Sprintf("%s-%s", d.namespace, task.ID),
	)
	if err != nil {
		return nil, fmt.Errorf("failed to create container: %w", err)
	}

	err = d.client.ContainerStart(ctx, response.ID, container.StartOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to start container: %w", err)
	}

	return &DockerContainer{
		id:     response.ID,
		client: d.client,
		task:   task,
	}, nil
}

func (d *DockerContainer) Status(ctx context.Context) (*DockerContainerStatus, error) {
	// doc: https://docs.docker.com/reference/api/engine/version/v1.43/#tag/Container/operation/ContainerInspect
	inspection, err := d.client.ContainerInspect(ctx, d.id)
	if err != nil {
		return nil, fmt.Errorf("failed to inspect container: %w", err)
	}

	return &DockerContainerStatus{
		state: inspection.State,
	}, nil
}

func (d *DockerContainer) Logs(ctx context.Context, stdout, stderr io.Writer) error {
	options := container.LogsOptions{
		ShowStdout: true,
		ShowStderr: true,
	}

	logs, err := d.client.ContainerLogs(ctx, d.id, options)
	if err != nil {
		return fmt.Errorf("failed to get container logs: %w", err)
	}

	_, err = stdcopy.StdCopy(stdout, stderr, logs)
	if err != nil {
		return fmt.Errorf("failed to copy logs: %w", err)
	}

	return nil
}

func (s *DockerContainerStatus) IsDone() bool {
	return s.state.Status == "exited"
}

func (s *DockerContainerStatus) ExitCode() int {
	return s.state.ExitCode
}
