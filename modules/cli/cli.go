package cli

import (
	"bytes"
	"errors"
	"fmt"
	"os/exec"
	"strconv"
	"strings"
)

type Cli struct{}

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

// ---------------------------------------------------------------------------------------------------------------------

// module interface implementation
func (cli Cli) Name() string {
	return "cli"
}

// tempExtractor interface implementation
func (cli Cli) GetTemp(cmd string) (temp float64, err error) {
	var tString string
	tString, err = CommandPipe(cmd)
	if err != nil {
		return
	}
	if tString == "" {
		err = fmt.Errorf("controller 'temp_cmd' returned an empty string: `%s`", cmd)
		return
	}

	tString = strings.Trim(tString, " .")
	return strconv.ParseFloat(tString, 64)
}
