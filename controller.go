package main

import (
	"math"
	"sort"
	"sync"
)

type targetData struct {
	dutyCycle          uint8
	lastUpdatedTemp    float64
	sortedMappingTemps []float64
}

// commanderpro, ipmi, cli
type controller struct {
	Name string `yaml:"name"`

	// commanderpro, ipmi, cli
	// commanderpro: sensor_channel (uint8 as string), ipmi: entityID, cli: custom_command
	Temp struct {
		Method string
		Arg    string
	} `yaml:"temp"`

	// MinTempChange is the minimum necessary change (in °C)
	// from the last duty cycle update to actually cause another update.
	MinTempChange float64 `yaml:"min_temp_change"`

	// Targets are the ipmi zone target with their temp/duty-cycle mapping.
	// cpu_zone: 0x00, io_zone: 0x01.
	// target : channel : mappings
	Targets map[string]map[float64]uint8 `yaml:"targets"`

	// runtime vars -----------------------------------

	once sync.Once

	// <target:channel> : targetData
	targetsData map[string]*targetData
}

// Calculate the needed duty-cycle for any target.
// Takes in consideration the last
// measured temp before the last update.
func (c *controller) getNeededDutyCycles(curTemp float64) map[string]*targetData {
	// prepare
	c.once.Do(func() {
		c.targetsData = make(map[string]*targetData)

		for target, mappings := range c.Targets {

			temps := make([]float64, 0)
			for t := range mappings {
				temps = append(temps, t)
			}
			sort.Float64s(temps)

			c.targetsData[target] = &targetData{
				dutyCycle:          0,
				lastUpdatedTemp:    0,
				sortedMappingTemps: temps,
			}
		}
	})

	for target, mappings := range c.Targets {

		lastTemp := c.targetsData[target].lastUpdatedTemp
		if math.Abs(curTemp-lastTemp) >= c.MinTempChange {

			c.targetsData[target].lastUpdatedTemp = curTemp

			for _, temp := range c.targetsData[target].sortedMappingTemps {

				if temp > curTemp {
					break
				}
				c.targetsData[target].dutyCycle = mappings[temp]
			}
		}
	}

	return c.targetsData
}
