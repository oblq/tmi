package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oblq/ipmifc/internal/exec"
	"gopkg.in/yaml.v3"
)

// FanMode is the ipmi fan mode.
type FanMode string

const (
	fanModeStandard FanMode = "00"
	fanModeFull     FanMode = "01"
	fanModeOptimal  FanMode = "02"
	fanModeHeavyIO  FanMode = "04"
	fanModeCustom   FanMode = "custom"
)

// IPMIFC is an ipmitool interface to handle fans duty-cycles.
type IPMIFC struct {
	mutex sync.Mutex

	ticker *time.Ticker

	running bool

	configPath, configThresholdsPath string
	configStat, configThresholdsStat os.FileInfo

	zonesDutyCycles map[string]uint8

	// CMD is the ipmitool preamble command,
	// could be act locally or on remote machines,
	// depending on the parameters, full example in config file.
	CMD string `yaml:"ipmi_cmd"`

	// FanMode is the ipmi fan mode.
	// Must be set to custom to use the configured controllers.
	FanMode FanMode `yaml:"fan_mode"`

	// CheckInterval is the time between checks, in seconds.
	CheckInterval int `yaml:"check_interval"`

	// Controllers is an array of controllers that will
	// define the overall behaviour of this program
	// depending on the configuration.
	Controllers []*controller `yaml:"controllers"`
}

// New return a new IPMIFC instance.
func New(path string) (ipmifc *IPMIFC, err error) {
	ipmifc = &IPMIFC{}

	ipmifc.configPath = filepath.Join(path, "ipmifc.yaml")
	ipmifc.configThresholdsPath = filepath.Join(path, "/ipmifc_thresholds.yaml")

	if ipmifc.configStat, err = os.Stat(ipmifc.configPath); err != nil {
		return
	}
	if ipmifc.configThresholdsStat, err = os.Stat(ipmifc.configThresholdsPath); err != nil {
		return
	}

	err = ipmifc.LoadConfig()

	// start watching config files
	watcherTicker := time.NewTicker(time.Second * time.Duration(ipmifc.CheckInterval))
	go func() {
		for range watcherTicker.C {
			ipmifc.checkConfigs()
		}
	}()

	return
}

// LoadConfig will do a hot reload of the
// program configuration.
func (ipmifc *IPMIFC) LoadConfig() error {
	ipmifc.mutex.Lock()

	ipmifc.Controllers = make([]*controller, 0)
	ipmifc.zonesDutyCycles = make(map[string]uint8)

	config, err := ioutil.ReadFile(ipmifc.configPath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(config, &ipmifc)
	if err != nil {
		return err
	}

	fmt.Println("config updated")

	ipmifc.StopMonitoring()

	switch ipmifc.FanMode {
	case fanModeStandard, fanModeFull, fanModeOptimal, fanModeHeavyIO:
		ipmifc.SetFanMode(ipmifc.FanMode)
		ipmifc.mutex.Unlock()

	case fanModeCustom:
		ipmifc.mutex.Unlock()

		currMode := ipmifc.GetFanMode()
		if currMode != string(fanModeFull) {
			ipmifc.SetFanMode(fanModeFull)
		}
		ipmifc.StartMonitoring()

	default:
		ipmifc.mutex.Unlock()
	}

	return nil
}

// LoadConfigThresholds will update ipmi fan thresholds.
// `sudo watch ipmitool sensor` to get the current settings.
func (ipmifc *IPMIFC) LoadConfigThresholds() error {
	ipmifc.mutex.Lock()
	defer ipmifc.mutex.Unlock()

	type FT struct {
		// FanThresholds are some custom fan thresholds,
		// Noctua fans needs this for instance.
		FanThresholds map[string]*fanThreshold `yaml:"fan_thresholds"`
	}

	var ft FT

	config, err := ioutil.ReadFile(ipmifc.configThresholdsPath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(config, &ft)
	if err != nil {
		return err
	}

	for name, fanThreshold := range ft.FanThresholds {
		fanThreshold.Name = name
		if err := fanThreshold.set(ipmifc.CMD); err != nil {
			fmt.Println("error setting fans threshold:", err.Error())
		}
	}

	return nil
}

// GetFanMode return the fan mode currently used by ipmi.
func (ipmifc *IPMIFC) GetFanMode() string {
	out, err := exec.Command(fmt.Sprintf("%s raw 0x30 0x45 0x00", ipmifc.CMD))
	if err != nil {
		fmt.Println("error getting fan mode", err.Error())
	}
	return strings.Trim(out, " ")
}

// SetFanMode set ipmi fan mode.
func (ipmifc *IPMIFC) SetFanMode(mode FanMode) {
	_, err := exec.Command(fmt.Sprintf("%s raw 0x30 0x45 0x01 %s", ipmifc.CMD, mode))
	if err != nil {
		fmt.Println("error setting fan mode to", mode, "->", err.Error())
	} else {
		fmt.Printf("fan mode set to %s\n", mode)
	}
}

// GetZoneDutyCycle return the passed zone duty-cycle.
func (ipmifc *IPMIFC) GetZoneDutyCycle(zone string) (uint8, error) {
	out, err := exec.Command(fmt.Sprintf("%s raw 0x30 0x70 0x66 0x00 %s", ipmifc.CMD, zone))
	if err != nil {
		return 0, fmt.Errorf("error getting duty cycle for zone '%s', err: %v", zone, err)
	}

	out = strings.Trim(out, " ")
	dc, err := strconv.ParseUint(out, 16, 32)
	return uint8(dc), err
}

// SetZoneDutyCycle set the passed duty-cycle for the given zone.
func (ipmifc *IPMIFC) SetZoneDutyCycle(zone string, dutyCycle uint8) error {
	cmdString := fmt.Sprintf("%s raw 0x30 0x70 0x66 0x01 %s %s", ipmifc.CMD, zone, strconv.Itoa(int(dutyCycle)))
	if _, err := exec.Command(cmdString); err != nil {
		return fmt.Errorf("error setting duty cycle for zone '%s' to %d%%, err: %v", zone, dutyCycle, err)
	}
	return nil
}

// StartMonitoring start the monitoring daemon,
// checking temps and duty-cycles.
func (ipmifc *IPMIFC) StartMonitoring() {
	if ipmifc.running {
		return
	}

	ipmifc.running = true
	ipmifc.check()

	ipmifc.ticker = time.NewTicker(time.Second * time.Duration(ipmifc.CheckInterval))
	go func() {
		for range ipmifc.ticker.C {
			ipmifc.check()
		}
	}()
}

// StopMonitoring stop the daemon.
func (ipmifc *IPMIFC) StopMonitoring() {
	if ipmifc.ticker != nil {
		ipmifc.ticker.Stop()
	}
	ipmifc.running = false
}

func (ipmifc *IPMIFC) check() {
	tempZonesDutyCycles := make(map[string]uint8)
	logString := "	| "

	ipmifc.mutex.Lock()

	// grab the greater values divided by zone first
	for _, controller := range ipmifc.Controllers {
		zdc, err := controller.getNeededDutyCycle(ipmifc.CMD)
		if err != nil {
			fmt.Println("error getting needed duty-cycle for", controller.Name, "->", err.Error())
		}
		logString += fmt.Sprintf("%s %5s | ", controller.Name, fmt.Sprint(controller.curTemp)+"°C")

		// grab the maximum needed dc value for every zone
		for zone, dc := range zdc {
			if dc > tempZonesDutyCycles[zone] {
				tempZonesDutyCycles[zone] = dc
			}
		}
	}

	// set the needed duty cycle if different from the current value
	for zone, dc := range tempZonesDutyCycles {
		if ipmifc.zonesDutyCycles[zone] != dc {
			ipmifc.zonesDutyCycles[zone] = dc

			//fmt.Printf("Updating '%s' zone duty cycle to: %d%%\n", zone, pwm)
			if err := ipmifc.SetZoneDutyCycle(zone, dc); err != nil {
				fmt.Println(err.Error())
			}
		} else {
			// correct misalignment
			realDC, err := ipmifc.GetZoneDutyCycle(zone)
			if err != nil {
				fmt.Println(err.Error())
			} else if realDC != dc {
				if err := ipmifc.SetZoneDutyCycle(zone, dc); err != nil {
					fmt.Println(err.Error())
				}
			}
		}
	}

	ipmifc.mutex.Unlock()

	logString += "	Zones duty-cycle: | "

	zones := make([]string, 0, len(ipmifc.zonesDutyCycles))
	for zone := range ipmifc.zonesDutyCycles {
		zones = append(zones, zone)
	}
	sort.Strings(zones)
	for _, zone := range zones {
		logString += fmt.Sprintf("%s %d%% | ", zone, ipmifc.zonesDutyCycles[zone])
	}

	fmt.Println(logString)
}

func (ipmifc *IPMIFC) checkConfigs() {
	if configStat, err := os.Stat(ipmifc.configPath); err != nil {
		fmt.Println("unable to stat config file:", err.Error())
	} else if configStat.Size() != ipmifc.configStat.Size() ||
		configStat.ModTime() != ipmifc.configStat.ModTime() {
		ipmifc.configStat = configStat
		_ = ipmifc.LoadConfig()
		return
	}

	if configThresholdsStat, err := os.Stat(ipmifc.configThresholdsPath); err != nil {
		fmt.Println("unable to stat config file:", err.Error())
	} else if configThresholdsStat.Size() != ipmifc.configThresholdsStat.Size() ||
		configThresholdsStat.ModTime() != ipmifc.configThresholdsStat.ModTime() {
		ipmifc.configThresholdsStat = configThresholdsStat
		_ = ipmifc.LoadConfigThresholds()
	}
}
