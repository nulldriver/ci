package orchestra

import (
	"context"
	"fmt"
	"io"

	"github.com/docker/docker/api/types"
	"github.com/docker/docker/api/types/container"
	"github.com/docker/docker/api/types/filters"
	"github.com/docker/docker/api/types/image"
	"github.com/docker/docker/client"
	"github.com/docker/docker/errdefs"
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

func NewDocker(namespace string) (Orchestrator, error) {
	cli, err := client.NewClientWithOpts(client.FromEnv, client.WithAPIVersionNegotiation())
	if err != nil {
		return nil, fmt.Errorf("failed to create docker client: %w", err)
	}

	return &Docker{
		client:    cli,
		namespace: namespace,
	}, nil
}

func (d *Docker) RunContainer(ctx context.Context, task Task) (Container, error) {
	reader, err := d.client.ImagePull(ctx, task.Image, image.PullOptions{})
	if err != nil {
		return nil, fmt.Errorf("failed to initiate pull image: %w", err)
	}

	_, err = io.Copy(io.Discard, reader)
	if err != nil {
		return nil, fmt.Errorf("failed to pull image: %w", err)
	}

	containerName := fmt.Sprintf("%s-%s", d.namespace, task.ID)

	response, err := d.client.ContainerCreate(
		ctx,
		&container.Config{
			Image: task.Image,
			Cmd:   task.Command,
		},
		nil, nil, nil,
		containerName,
	)
	if err != nil {
		if errdefs.IsConflict(err) {
			filter := filters.NewArgs()
			filter.Add("name", containerName)

			containers, err := d.client.ContainerList(ctx, container.ListOptions{Filters: filter, All: true})
			if err != nil {
				return nil, fmt.Errorf("failed to list containers: %w", err)
			}

			if len(containers) == 0 {
				return nil, fmt.Errorf("failed to find container by name %s", containerName)
			}

			return &DockerContainer{
				id:     containers[0].ID,
				client: d.client,
				task:   task,
			}, nil
		}
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

func (d *DockerContainer) Status(ctx context.Context) (ContainerStatus, error) {
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

func (d *DockerContainer) Cleanup(ctx context.Context) error {
	err := d.client.ContainerRemove(ctx, d.id, container.RemoveOptions{
		Force:         true,
		RemoveLinks:   false,
		RemoveVolumes: false,
	})
	if err != nil {
		return fmt.Errorf("failed to remove container: %w", err)
	}

	return nil
}

func (s *DockerContainerStatus) IsDone() bool {
	return s.state.Status == "exited"
}

func (s *DockerContainerStatus) ExitCode() int {
	return s.state.ExitCode
}

func init() {
	Add("docker", NewDocker)
}
