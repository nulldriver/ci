package commands

import (
	"context"
	"fmt"
	"io"
	"log/slog"
	"os"
	"strings"

	"github.com/dop251/goja"
	"github.com/dop251/goja_nodejs/console"
	"github.com/dop251/goja_nodejs/require"
	"github.com/evanw/esbuild/pkg/api"
	"github.com/google/uuid"
	"github.com/jtarchie/ci/orchestra"
)

type Runtime struct {
	Pipeline     *os.File `arg:"" help:"Path to pipeline javascript file"`
	Orchestrator string   `help:"orchestrator runtime to use" default:"native"`
}

type Result struct {
	Code   int    `json:"code" js:"code"`
	Error  string `json:"error" js:"error"`
	Stderr string `json:"stderr" js:"stderr"`
	Stdout string `json:"stdout" js:"stdout"`
}

type RunInput struct {
	Command []string `json:"command" js:"command"`
	Image   string   `json:"image" js:"image"`
	Name    string   `json:"name" js:"name"`
}

func (c *Runtime) Run() error {
	contents, err := io.ReadAll(c.Pipeline)
	if err != nil {
		return fmt.Errorf("failed to read pipeline file: %w", err)
	}

	orchestrator, found := orchestra.Get(c.Orchestrator)
	if !found {
		return fmt.Errorf("could not get orchestrator: %s", c.Orchestrator)
	}

	client, err := orchestrator("ci")
	if err != nil {
		return fmt.Errorf("could not create docker client: %w", err)
	}

	// this is setup to build the pipeline in a goja vm
	vm := goja.New()
	vm.SetFieldNameMapper(goja.TagFieldNameMapper("json", true))

	new(require.Registry).Enable(vm)
	console.Enable(vm)

	run := func(input RunInput) *Result {
		id, err := uuid.NewV7()
		if err != nil {
			return &Result{
				Code:  1,
				Error: fmt.Sprintf("could not generate uuid: %s", err),
			}
		}

		logger := slog.With("id", id, "orchestrator", c.Orchestrator)

		logger.Info("container.run", "input", input)

		container, err := client.RunContainer(
			context.Background(),
			orchestra.Task{
				ID:      fmt.Sprintf("%s-%s", input.Name, id.String()),
				Image:   input.Image,
				Command: input.Command,
			},
		)
		if err != nil {
			return &Result{
				Code:  1,
				Error: fmt.Sprintf("could not run container: %s", err),
			}
		}

		var status orchestra.ContainerStatus

		for {
			var err error

			status, err = container.Status(context.Background())
			if err != nil {
				return &Result{
					Code:  1,
					Error: fmt.Sprintf("could not get container status: %s", err),
				}
			}

			if status.IsDone() {
				break
			}
		}

		logger.Info("container.status", "exitCode", status.ExitCode())

		defer func() {
			err := container.Cleanup(context.Background())
			if err != nil {
				slog.Error("container.cleanup", "err", err)
			}
		}()

		stdout, stderr := &strings.Builder{}, &strings.Builder{}
		err = container.Logs(context.Background(), stdout, stderr)
		if err != nil {
			logger.Error("container.logs", "err", err)

			return &Result{
				Code:  status.ExitCode(),
				Error: fmt.Sprintf("could not get container logs: %s", err),
			}
		}

		return &Result{
			Stdout: stdout.String(),
			Stderr: stderr.String(),
			Code:   status.ExitCode(),
		}
	}

	err = vm.Set("run", run)
	if err != nil {
		return fmt.Errorf("could not set run function: %w", err)
	}

	result := api.Transform(string(contents), api.TransformOptions{
		Loader:    api.LoaderTS,
		Format:    api.FormatCommonJS,
		Target:    api.ES2015,
		Sourcemap: api.SourceMapNone,
		Platform:  api.PlatformNeutral,
	})

	if len(result.Errors) > 0 {
		return &goja.CompilerSyntaxError{
			CompilerError: goja.CompilerError{
				Message: result.Errors[0].Text,
			},
		}
	}

	program, err := goja.Compile(
		"main.js",
		"{(function() { const module = {}; "+string(result.Code)+"; return module.exports.pipeline;}).apply(undefined)}",
		true,
	)
	if err != nil {
		return fmt.Errorf("could not compile: %w", err)
	}

	pipeline, err := vm.RunProgram(program)
	if err != nil {
		defer vm.ClearInterrupt()

		return fmt.Errorf("could not run program: %w", err)
	}

	// let's run the pipeline
	pipelineFunc, ok := goja.AssertFunction(pipeline)
	if !ok {
		return fmt.Errorf("pipeline is not a function")
	}

	_, err = pipelineFunc(goja.Undefined())
	if err != nil {
		return fmt.Errorf("could not run pipeline: %w", err)
	}

	return nil
}
