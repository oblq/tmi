package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strings"
)

func command(cmdString string) (string, error) {
	nameCmd := strings.SplitN(cmdString, " ", 2)
	if len(nameCmd) != 2 {
		return "", errors.New("wrong cmd: " + cmdString)
	}

	executable := nameCmd[0]
	args := strings.Fields(nameCmd[1])
	for i, arg := range args {
		arg = strings.TrimPrefix(arg, "'")
		arg = strings.TrimSuffix(arg, "'")
		args[i] = arg
	}

	cmd := exec.Command(executable, args...)
	// If Env is nil, the new process uses the current process's environment.
	cmd.Env = os.Environ()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	var stout bytes.Buffer
	cmd.Stdout = &stout

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, stderr.String())
	}

	out := strings.TrimSuffix(stout.String(), "\n")
	return out, nil
}

func commandPipe(cmdString string) (string, error) {
	cmd := exec.Command("bash", "-c", cmdString)
	// If Env is nil, the new process uses the current process's environment.
	cmd.Env = os.Environ()

	var stderr bytes.Buffer
	cmd.Stderr = &stderr

	var stout bytes.Buffer
	cmd.Stdout = &stout

	err := cmd.Run()
	if err != nil {
		return "", fmt.Errorf("%v: %s", err, stderr.String())
	}

	out := strings.TrimSuffix(stout.String(), "\n")
	return out, nil
}
