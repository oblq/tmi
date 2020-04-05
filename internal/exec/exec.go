package exec

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strings"
)

func Command(cmdString string) (string, error) {
	nameCmd := strings.SplitN(cmdString, " ", 2)
	if len(nameCmd) != 2 {
		return "", errors.New("wrong cmd: " + cmdString)
	}

	name := nameCmd[0]
	arg := strings.Fields(nameCmd[1])

	cmd := exec.Command(name, arg...)

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

func CommandPipe(cmdString string) (string, error) {
	cmd := exec.Command("bash", "-c", cmdString)

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
