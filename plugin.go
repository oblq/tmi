package main

// TMIPluginGetter is the func that the go plugin (<plugin>.so) must implement
// https://stackoverflow.com/a/47399371/3079922
type TMIPluginGetter = func() interface{}

type TMIPlugin interface {
	Name() string
	ReadConfig(path string)
	ShutDown()
}

type TMITempExtractor interface {
	TMIPlugin
	GetTemp(arg string) (temp float64, err error)
}

type TMIFanController interface {
	TMIPlugin
	GetChannelDutyCycle(ch uint8) (dc uint8, err error)
	SetChannelDutyCycle(ch uint8, dc uint8) error
}
