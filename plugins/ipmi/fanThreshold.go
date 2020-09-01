package main

import (
	"errors"
	"fmt"
)

type fanThreshold struct {
	Name        string   `yaml:"name"`
	Description string   `yaml:"description"`
	Lower       []string `yaml:"lower"`
	Upper       []string `yaml:"upper"`
}

func (t *fanThreshold) set(ipmiCMD string) error {
	if len(t.Lower) < 3 {
		return errors.New("lower thresholds must have three values: Non-Recoverable, Critical and Non-Critical")
	}

	if len(t.Upper) < 3 {
		return errors.New("upper thresholds must have three values: Non-Critical, Critical and Non-Recoverable")
	}

	var err error
	var out string

	cmdLower := fmt.Sprintf("%s sensor thresh %s lower %s %s %s",
		ipmiCMD, t.Name, t.Lower[0], t.Lower[1], t.Lower[2])

	out, err = command(cmdLower)
	if err != nil {
		return err
	}

	fmt.Println(out)

	cmdUpper := fmt.Sprintf("%s sensor thresh %s upper %s %s %s",
		ipmiCMD, t.Name, t.Upper[0], t.Upper[1], t.Upper[2])

	out, err = command(cmdUpper)
	if err != nil {
		return err
	}

	fmt.Println(out)
	return nil
}
