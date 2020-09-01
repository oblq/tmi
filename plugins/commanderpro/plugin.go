package main

// TMIPluginGetter is the func that the go plugin (<plugin>.so) must implement
type TMIPluginGetter = func() TMIPlugin

type TMIPlugin interface {
	Name() string
	ReadConfig(path string)
	ShutDown()
}

type TMITempExtractor interface {
	TMIPlugin
	GetTemp(arg string) (temp float64, err error)
}
