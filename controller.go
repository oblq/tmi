package main

import (
	"fmt"
	"math"
	"strconv"
	"strings"

	"github.com/oblq/ipmifc/internal/exec"
)

const fallbackDutyCycle = 50 // 50%

type controller struct {
	Name string `yaml:"name"`

	// TempCmd is a direct command to get a temp in number format,
	// take precedence over 'temp' parameter.
	TempCmd string `yaml:"temp_cmd"`

	// TempIpmiEntity is the ipmi entityID to look for in `ipmitool sdr list full` response.
	TempIpmiEntity string `yaml:"temp_ipmi_entity"`

	// MinTempChange is the minimum necessary change (in °C)
	// from the last duty cycle update to actually cause another update.
	MinTempChange float64 `yaml:"min_temp_change"`

	// Targets are the ipmi zone target with their temp/duty-cycle mapping.
	// cpu_zone: 0x00, io_zone: 0x01.
	Targets map[string]map[float64]uint8 `yaml:"targets"`

	curTemp               float64
	lastTemp              float64
	zonesNeededDutyCycles map[string]uint8
}

// fallbackDutyCycle is returned on error
func (c *controller) getNeededDutyCycle(ipmiCMD string) (zonesDutyCycles map[string]uint8, err error) {

	if c.zonesNeededDutyCycles == nil {
		c.zonesNeededDutyCycles = make(map[string]uint8)
	}

	var temp string

	// to return with errors...
	zonesDutyCycles = make(map[string]uint8)
	for zone := range c.Targets {
		zonesDutyCycles[zone] = fallbackDutyCycle
	}

	if c.TempCmd == "" {
		cmdString := fmt.Sprintf("%s sdr entity %s | cut -d '|' -f 5 | cut -d ' ' -f2",
			ipmiCMD, c.TempIpmiEntity)
		temp, err = exec.CommandPipe(cmdString)
		if err != nil {
			return
		}
		if temp == "" {
			err = fmt.Errorf("entityID not found: %s", c.TempIpmiEntity)
			return
		}
	} else {
		temp, err = exec.CommandPipe(c.TempCmd)
		if err != nil {
			return
		}
		if temp == "" {
			err = fmt.Errorf("controller 'temp_cmd' returned an empty string: `%s`", c.TempCmd)
			return
		}
	}

	temp = strings.Trim(temp, " .")
	c.curTemp, err = strconv.ParseFloat(temp, 64)
	if err != nil {
		return
	}

	if math.Abs(c.curTemp-c.lastTemp) >= c.MinTempChange {
		for zone, curveMappings := range c.Targets {
			for temp, dc := range curveMappings {
				if c.curTemp >= temp {
					c.zonesNeededDutyCycles[zone] = dc
					c.lastTemp = c.curTemp
				}
			}
		}
	}

	return c.zonesNeededDutyCycles, nil
}
