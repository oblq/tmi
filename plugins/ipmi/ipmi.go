package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"strconv"
	"strings"

	"gopkg.in/yaml.v3"
)

func main() {

}

// fanMode is the ipmi fan mode.
type fanMode string

const (
	FanModeStandard fanMode = "00"
	FanModeFull     fanMode = "01"
	FanModeOptimal  fanMode = "02"
	FanModeHeavyIO  fanMode = "04"
	FanModeCustom   fanMode = "custom"
)

// IPMI is an ipmitool interface to handle fans duty-cycles.
type IPMI struct {
	//configPath string
	configStat os.FileInfo

	zonesDutyCycles map[string]uint8

	// CMD is the ipmitool preamble command,
	// could run locally or on remote machines,
	// depending on the parameters, full example in config file.
	CMD string `yaml:"cmd"`

	// FanThresholds are some custom fan thresholds,
	// Noctua fans needs this for instance.
	FanThresholds map[string]*fanThreshold `yaml:"fan_thresholds"`

	// fanMode is the ipmi fan mode.
	// Must be set to custom to use the configured controllers.
	//fanMode fanMode `yaml:"fan_mode"`
}

// New return a new IPMI instance.
func New() (ipmi *IPMI, err error) {
	ipmi = &IPMI{zonesDutyCycles: make(map[string]uint8)}

	//err = ipmi.LoadConfig()

	return
}

// GetFanMode return the fan mode currently used by ipmi.
func (ipmi *IPMI) GetFanMode() string {
	out, err := command(fmt.Sprintf("%s raw 0x30 0x45 0x00", ipmi.CMD))
	if err != nil {
		fmt.Println("error getting fan mode", err.Error())
	}
	return strings.Trim(out, " ")
}

// SetFanMode set ipmi fan mode.
func (ipmi *IPMI) SetFanMode(mode fanMode) {
	_, err := command(fmt.Sprintf("%s raw 0x30 0x45 0x01 %s", ipmi.CMD, mode))
	if err != nil {
		fmt.Println("error setting fan mode to", mode, "->", err.Error())
	} else {
		fmt.Printf("fan mode set to %s\n", mode)
	}
}

// pluggableModule interface implementation ----------------------------------------------------------------------------

func Plugin() interface{} {
	ipmi, err := New()
	if err != nil {
		panic(err)
	}
	return ipmi
}

func (ipmi *IPMI) Name() string {
	return "ipmi"
}

func (ipmi *IPMI) ReadConfig(configPath string) {
	path := filepath.Join(configPath, "ipmi.yaml")
	if configStat, err := os.Stat(path); err != nil {
		fmt.Println("unable to stat config file:", err.Error())
	} else if ipmi.configStat == nil ||
		configStat.Size() != ipmi.configStat.Size() ||
		configStat.ModTime() != ipmi.configStat.ModTime() {

		ipmi.configStat = configStat

		config, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		err = yaml.Unmarshal(config, &ipmi)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		// update ipmi fan thresholds,
		// `sudo watch ipmitool sensor` to get the current settings.
		for name, fanThreshold := range ipmi.FanThresholds {
			fanThreshold.Name = name
			if err := fanThreshold.set(ipmi.CMD); err != nil {
				fmt.Println("error setting fans threshold:", err.Error())
			}
		}

		currMode := ipmi.GetFanMode()
		if currMode != string(FanModeFull) {
			ipmi.SetFanMode(FanModeFull)
		}

		fmt.Println("ipmi config updated")
	}
}

func (ipmi *IPMI) ShutDown() {}

func (ipmi *IPMI) GetTemp(entityID string) (temp float64, err error) {
	var tString string
	cmdString := fmt.Sprintf("%s sdr entity %s | cut -d '|' -f 5 | cut -d ' ' -f2", ipmi.CMD, entityID)
	tString, err = commandPipe(cmdString)
	if err != nil {
		return
	}
	if tString == "" {
		err = fmt.Errorf("entityID not found: `%s`", entityID)
		return
	}

	tString = strings.Trim(tString, " .")
	return strconv.ParseFloat(tString, 32)
}

// GetZoneDutyCycle return the passed zone duty-cycle.
func (ipmi *IPMI) GetChannelDutyCycle(ch uint8) (uint8, error) {
	cmdString := fmt.Sprintf("%s raw 0x30 0x70 0x66 0x00 %#02x", ipmi.CMD, ch)
	out, err := command(cmdString)
	if err != nil {
		return 0, fmt.Errorf("error getting duty cycle for channel '%v', err: %v", ch, err)
	}

	out = strings.Trim(out, " ")
	dc, err := strconv.ParseUint(out, 16, 8)
	return uint8(dc), err
}

// SetZoneDutyCycle set the passed duty-cycle for the given zone.
func (ipmi *IPMI) SetChannelDutyCycle(ch uint8, dc uint8) error {
	cmdString := fmt.Sprintf("%s raw 0x30 0x70 0x66 0x01 %#02x %#02x", ipmi.CMD, ch, dc)
	if _, err := command(cmdString); err != nil {
		return fmt.Errorf("error setting duty cycle for channel '%v' to %d%%, err: %v", ch, dc, err)
	}
	return nil
}
