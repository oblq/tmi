package main

import (
	"testing"

	"github.com/stretchr/testify/require"
)

func Test_controller_getNeededDutyCycles(t *testing.T) {
	sortedTemps := []float64{0, 10, 20, 30, 40, 50}

	tests := []struct {
		name string
		temp float64
		want map[string]*targetData
	}{
		{name: "a", temp: 5, want: map[string]*targetData{"t1": {dutyCycle: 0, lastUpdatedTemp: 5, sortedMappingTemps: sortedTemps}}},
		{name: "b", temp: 10, want: map[string]*targetData{"t1": {dutyCycle: 10, lastUpdatedTemp: 10, sortedMappingTemps: sortedTemps}}},
		{name: "c", temp: 9, want: map[string]*targetData{"t1": {dutyCycle: 10, lastUpdatedTemp: 10, sortedMappingTemps: sortedTemps}}},
		{name: "d", temp: 8, want: map[string]*targetData{"t1": {dutyCycle: 0, lastUpdatedTemp: 8, sortedMappingTemps: sortedTemps}}},
		{name: "e", temp: 11, want: map[string]*targetData{"t1": {dutyCycle: 10, lastUpdatedTemp: 11, sortedMappingTemps: sortedTemps}}},
		{name: "f", temp: 30, want: map[string]*targetData{"t1": {dutyCycle: 30, lastUpdatedTemp: 30, sortedMappingTemps: sortedTemps}}},
		{name: "g", temp: 5, want: map[string]*targetData{"t1": {dutyCycle: 0, lastUpdatedTemp: 5, sortedMappingTemps: sortedTemps}}},
	}

	c := &controller{
		MinTempChange: 2,
		Targets: map[string]map[float64]uint8{
			"t1": {
				10: 10,
				0:  0,
				30: 30,
				20: 20,
				50: 50,
				40: 40,
			},
		},
	}

	for _, tt := range tests {
		t.Run(tt.name, func(t *testing.T) {
			got := c.getNeededDutyCycles(tt.temp)
			require.Equal(t, tt.want, got)
		})
	}
}
