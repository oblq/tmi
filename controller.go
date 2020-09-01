package main

import (
	"math"
	"sort"
	"sync"
)

type controller struct {
	Name string `yaml:"name"`

	Temp struct {
		Plugin string
		Arg    string
	} `yaml:"temp"`

	// MinTempChange is the minimum necessary change (in °C)
	// from the last duty cycle update to actually cause another update.
	MinTempChange float64 `yaml:"min_temp_change"`

	// Targets are the temp/duty-cycle mapping for any given target derived from 'targets_map'.
	// targets_map:
	//  pump: ipmi.0
	//
	// targets:
	//      pump:
	//        0:  30
	//        36: 50
	Targets map[string]map[float64]uint8 `yaml:"targets"`

	// runtime vars -----------------------------------------------------------

	once sync.Once

	// target derived from 'targets_map'.
	// targets_map:
	//  pump: ipmi.0
	//
	// <target> : *targetData
	targets map[string]*target
}

func (c *controller) prepare() {
	c.once.Do(func() {
		c.targets = make(map[string]*target)

		for targetName, mappings := range c.Targets {
			temps := make([]float64, 0)
			for t := range mappings {
				temps = append(temps, t)
			}
			sort.Float64s(temps)

			c.targets[targetName] = &target{
				dutyCycle:          0,
				lastUpdatedTemp:    0,
				sortedMappingTemps: temps,
			}
		}
	})
}

// Calculate the needed duty-cycle for any target.
// Takes in consideration the last
// measured temp before the last update.
func (c *controller) getNeededDutyCycles(curTemp float64) map[string]*target {
	c.prepare()

	for target, mappings := range c.Targets {

		lastTemp := c.targets[target].lastUpdatedTemp
		if math.Abs(curTemp-lastTemp) >= c.MinTempChange {

			c.targets[target].lastUpdatedTemp = curTemp

			for _, temp := range c.targets[target].sortedMappingTemps {

				if temp > curTemp {
					break
				}
				c.targets[target].dutyCycle = mappings[temp]
			}
		}
	}

	return c.targets
}
