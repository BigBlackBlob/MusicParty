//go:build windows

package media

import (
	"bytes"
	"context"
	"fmt"
	"os/exec"
	"strconv"
)

func runProcessTree(ctx context.Context, executable string, arguments ...string) error {
	cmd := command(ctx, executable, arguments...)
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		killer := exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F")
		_ = killer.Run()
		<-done
		return fmt.Errorf("%w", ctx.Err())
	}
}

func runProcessTreeOutput(ctx context.Context, executable string, arguments ...string) ([]byte, error) {
	var output bytes.Buffer
	cmd := command(ctx, executable, arguments...)
	cmd.Stdout = &output
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return output.Bytes(), err
	case <-ctx.Done():
		_ = exec.Command("taskkill", "/PID", strconv.Itoa(cmd.Process.Pid), "/T", "/F").Run()
		<-done
		return nil, fmt.Errorf("%w", ctx.Err())
	}
}
