//go:build !windows

package media

import (
	"bytes"
	"context"
	"fmt"
	"syscall"
)

func runProcessTree(ctx context.Context, executable string, arguments ...string) error {
	cmd := command(ctx, executable, arguments...)
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return err
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		return fmt.Errorf("%w", ctx.Err())
	}
}

func runProcessTreeOutput(ctx context.Context, executable string, arguments ...string) ([]byte, error) {
	var output bytes.Buffer
	cmd := command(ctx, executable, arguments...)
	cmd.Stdout = &output
	cmd.SysProcAttr = &syscall.SysProcAttr{Setpgid: true}
	if err := cmd.Start(); err != nil {
		return nil, err
	}
	done := make(chan error, 1)
	go func() { done <- cmd.Wait() }()
	select {
	case err := <-done:
		return output.Bytes(), err
	case <-ctx.Done():
		_ = syscall.Kill(-cmd.Process.Pid, syscall.SIGKILL)
		<-done
		return nil, fmt.Errorf("%w", ctx.Err())
	}
}
