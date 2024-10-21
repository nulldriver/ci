package main

import (
	"fmt"
	"io"
	"log/slog"
	"os"

	"github.com/alecthomas/kong"
)

type CLI struct{
	Pipeline *os.File `arg:"" help:"Path to pipeline javascript file"`
}

func main() {
	slog.SetDefault(slog.New(slog.NewJSONHandler(os.Stderr, nil)))

	cli := &CLI{}
	ctx := kong.Parse(cli)
	// Call the Run() method of the selected parsed command.
	err := ctx.Run()
	ctx.FatalIfErrorf(err)
}

func (c *CLI) Run() error {
	contents, err := io.ReadAll(c.Pipeline)
	if err != nil {
		return fmt.Errorf("failed to read pipeline file: %w", err)
	}

	
	return nil
}