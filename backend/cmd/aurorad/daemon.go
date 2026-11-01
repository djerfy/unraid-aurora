package main

import (
	"fmt"
	"os"
	"os/exec"
	"syscall"
)

const daemonChildEnv = "AURORA_DAEMON_CHILD"

func isDaemonChild() bool {
	return os.Getenv(daemonChildEnv) == "1"
}

// daemonize re-executes the current binary in a new session so the caller can
// exit immediately. Standard streams are redirected to the log file (or
// /dev/null), which is also what a service manager would otherwise provide.
func daemonize(logFile string) error {
	executable, err := os.Executable()
	if err != nil {
		return fmt.Errorf("resolve executable: %w", err)
	}

	devNull, err := os.OpenFile(os.DevNull, os.O_RDWR, 0)
	if err != nil {
		return fmt.Errorf("open %s: %w", os.DevNull, err)
	}
	defer devNull.Close()

	output := devNull
	if logFile != "" {
		handle, err := openLogFile(logFile)
		if err != nil {
			return err
		}
		defer handle.Close()
		output = handle
	}

	command := exec.Command(executable, os.Args[1:]...)
	command.Env = append(os.Environ(), daemonChildEnv+"=1")
	command.Stdin = devNull
	command.Stdout = output
	command.Stderr = output
	command.SysProcAttr = &syscall.SysProcAttr{Setsid: true}

	if err := command.Start(); err != nil {
		return fmt.Errorf("start background process: %w", err)
	}
	fmt.Fprintf(os.Stdout, "aurora started in background (pid %d)\n", command.Process.Pid)
	return nil
}
