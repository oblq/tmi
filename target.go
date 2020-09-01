package main

type target struct {
	dutyCycle uint8

	// only used from Controller
	lastUpdatedTemp    float64
	sortedMappingTemps []float64

	// only used from TMI
	fanController TMIFanController
	channel       uint8
}
