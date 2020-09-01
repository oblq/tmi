package main

import (
	"bytes"
	"errors"
	"fmt"
	"os"
	"os/exec"
	"strconv"
	"strings"
)

func main() {

}

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

func CommandPipe(cmdString string) (string, error) {
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

// pluggableModule interface implementation ----------------------------------------------------------------------------

func Plugin() interface{} {
	return &Cli{}
}

type Cli struct{}

func (cli Cli) Name() string {
	return "cli"
}

func (cli Cli) ReadConfig(_ string) {}

func (cli Cli) ShutDown() {}

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

//func (cli Cli) GetChannelDutyCycle(ch uint8) (dc uint8, err error) {
//	return 0, errors.New("cli module does not implement the fanController interface")
//}
//func (cli Cli) SetChannelDutyCycle(ch uint8, dc uint8) error {
//	return errors.New("cli module does not implement the fanController interface")
//}
