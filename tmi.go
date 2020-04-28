package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"github.com/oblq/tmi/modules/cli"
	"github.com/oblq/tmi/modules/commanderpro"
	"github.com/oblq/tmi/modules/ipmi"
	"gopkg.in/yaml.v3"
)

type module interface {
	Name() string
}

type tempExtractor interface {
	module
	GetTemp(arg string) (temp float64, err error)
}

type fanController interface {
	module
	SetChannelDutyCycle(ch uint8, dc uint8) error
	GetChannelDutyCycle(ch uint8) (dc uint8, err error)
	CheckConfig(path string)
}

type closer interface {
	module
	Close()
}

// ---------------------------------------------------------------------------------------------------------------------

type Target struct {
	fanController fanController
	channel       uint8
}

type ControlManager struct {
	mutex   sync.Mutex
	ticker  *time.Ticker
	running bool

	configPath string
	configStat os.FileInfo

	ActiveModules struct {
		Ipmi         bool
		CommanderPro bool
	} `yaml:"active_modules"`

	// checkInterval is the time between checks, in seconds.
	CheckInterval int `yaml:"check_interval"`

	tempGetters    map[string]tempExtractor
	fanControllers map[string]fanController
	// modules that needs to be closed
	closers map[string]closer

	// a map containing arbitrary names associated with a fanController:channel couple.
	// eg.: `pump: ipmi.0` or `side: commanderpro.2`
	TargetsMap map[string]string `yaml:"targets_map"`
	// prepared target list with parsed fanController and channel
	targets map[string]Target

	// Controllers is an array of controllers.
	// A controller consist in a struct describing the
	// method to be used to obtain the temperature
	// and the temp/duty-cycle mapping to be used for
	// one or most specified targets.
	Controllers []*controller `yaml:"controllers"`

	// targetsDutyCycle represent the currently used
	// duty-cycle for any given target.
	targetsDutyCycle map[string]uint8
}

func New(configPath string) (cm *ControlManager, err error) {
	cm = &ControlManager{
		configPath:       configPath,
		tempGetters:      make(map[string]tempExtractor),
		fanControllers:   make(map[string]fanController),
		closers:          make(map[string]closer),
		targets:          make(map[string]Target),
		Controllers:      make([]*controller, 0),
		targetsDutyCycle: make(map[string]uint8),
	}

	cliInterface := &cli.Cli{}
	cm.addModule(cliInterface)

	return
}

func (cm *ControlManager) addModule(module interface{}) {
	if te, ok := module.(tempExtractor); ok {
		cm.tempGetters[te.Name()] = te
	}

	if fc, ok := module.(fanController); ok {
		cm.fanControllers[fc.Name()] = fc
	}

	if c, ok := module.(closer); ok {
		cm.closers[c.Name()] = c
	}
}

func (cm *ControlManager) hasModule(moduleName string) bool {
	if _, ok := cm.tempGetters[moduleName]; ok {
		return true
	}
	if _, ok := cm.fanControllers[moduleName]; ok {
		return true
	}
	return false
}

// LoadConfig will do a hot reload of the
// program configuration.
func (cm *ControlManager) LoadConfigAndStart() (err error) {
	cm.mutex.Lock()

	cm.StopMonitoring()

	configPath := filepath.Join(cm.configPath, "tmi.yaml")
	if cm.configStat, err = os.Stat(configPath); err != nil {
		return
	}

	config, err := ioutil.ReadFile(configPath)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(config, &cm)
	if err != nil {
		return
	}

	fmt.Println("config updated")

	// update modules
	if cm.ActiveModules.Ipmi && !cm.hasModule("ipmi") {
		var ipmiInterface *ipmi.IPMI
		ipmiInterface, err = ipmi.New()
		if err != nil {
			return
		}
		cm.addModule(ipmiInterface)
	}

	if cm.ActiveModules.CommanderPro && !cm.hasModule("commanderpro") {
		var cpInterface *commanderpro.CommanderPro
		cpInterface, err = commanderpro.Open()
		if err != nil {
			return fmt.Errorf("unable to open connection to Corsair Commander Pro: " + err.Error())
		}
		cpInterface.GetExternalTemp = func(method, arg string) (temp float64, err error) {
			tg, ok := cm.tempGetters[method]
			if !ok {
				return 0, fmt.Errorf("no such temp method: %s, defined in commanderpro config", method)
			}

			return tg.GetTemp(arg)
		}
		cm.addModule(cpInterface)
	}

	// reset values
	cm.targetsDutyCycle = make(map[string]uint8)

	err = cm.parseTargetsMap()

	cm.mutex.Unlock()

	for _, fc := range cm.fanControllers {
		fc.CheckConfig(cm.configPath)
	}

	cm.StartMonitoring()

	return
}

func (cm *ControlManager) parseTargetsMap() (err error) {
	for key, couple := range cm.TargetsMap {
		coupleArr := strings.SplitN(couple, ".", 2)
		if len(coupleArr) < 2 {
			err = errors.New("target_map value must be a string containing the target and one of its channels separated by a dot. eg.: `ipmi.0` ")
			return
		}
		fanControllerName := coupleArr[0]
		fanController, ok := cm.fanControllers[fanControllerName]
		if !ok {
			return fmt.Errorf("no such target %s", fanControllerName)
		}
		var fanControllerChannel int
		fanControllerChannel, err = strconv.Atoi(coupleArr[1])
		if err != nil {
			return fmt.Errorf("target channel for %s is not a valid integer", fanControllerName)
		}
		cm.targets[key] = Target{
			fanController: fanController,
			channel:       uint8(fanControllerChannel),
		}
	}

	return
}

func (cm *ControlManager) checkConfig() {
	configPath := filepath.Join(cm.configPath, "tmi.yaml")
	if configStat, err := os.Stat(configPath); err != nil {
		fmt.Println("unable to stat config file:", err.Error())
	} else if cm.configStat == nil ||
		configStat.Size() != cm.configStat.Size() || configStat.ModTime() != cm.configStat.ModTime() {
		cm.configStat = configStat

		if err := cm.LoadConfigAndStart(); err != nil {
			fmt.Println(err.Error())
		}

		return
	}

	for _, fc := range cm.fanControllers {
		fc.CheckConfig(cm.configPath)
	}
}

// StartMonitoring start the monitoring daemon,
// checking temps and duty-cycles.
func (cm *ControlManager) StartMonitoring() {
	if cm.running {
		return
	}

	cm.running = true
	cm.check()

	if cm.ticker != nil {
		cm.ticker.Stop()
	}
	cm.ticker = time.NewTicker(time.Second * time.Duration(cm.CheckInterval))
	go func() {
		for range cm.ticker.C {
			cm.check()
			cm.checkConfig()
		}
	}()
}

// StopMonitoring stop the daemon.
func (cm *ControlManager) StopMonitoring() {
	if cm.ticker != nil {
		cm.ticker.Stop()
	}
	cm.running = false
}

func (cm *ControlManager) check() {
	logString := "	| "

	cm.mutex.Lock()

	// grab the greater values divided by zone first
	tempTargetsDutyCycles := make(map[string]uint8)
	for _, controller := range cm.Controllers {
		tg, ok := cm.tempGetters[controller.Temp.Method]
		if !ok {
			fmt.Println("no such temp method: " + controller.Temp.Method)
			continue
		}

		temp, err := tg.GetTemp(controller.Temp.Arg)
		if err != nil {
			fmt.Println("error getting temperature for", controller.Name, "->", err.Error())
			continue
		}

		logString += fmt.Sprintf("%s %5s | ", controller.Name, fmt.Sprint(temp)+"°C")

		// grab the maximum needed dc value for every target
		for target, targetData := range controller.getNeededDutyCycles(temp) {
			if targetData.dutyCycle >= tempTargetsDutyCycles[target] {
				tempTargetsDutyCycles[target] = targetData.dutyCycle
			}
		}
	}

	// set the needed duty cycle if different from the current value
	for target, dc := range tempTargetsDutyCycles {
		t, ok := cm.targets[target]
		if !ok {
			fmt.Println("no such target: " + target)
			continue
		}

		if dc == 0 || cm.targetsDutyCycle[target] != dc {
			cm.targetsDutyCycle[target] = dc

			//fmt.Printf("Updating '%s' zone duty cycle to: %d%%\n", zone, pwm)
			if err := t.fanController.SetChannelDutyCycle(t.channel, dc); err != nil {
				fmt.Println(err.Error())
			}

		} else {
			// correct misalignment
			realDC, err := t.fanController.GetChannelDutyCycle(t.channel)
			if err != nil {
				fmt.Println(err.Error())
			} else if realDC != dc {
				if err := t.fanController.SetChannelDutyCycle(t.channel, dc); err != nil {
					fmt.Println(err.Error())
				}
			}
		}
	}

	cm.mutex.Unlock()

	logString += "	->	| "

	targets := make([]string, 0)
	for target, dc := range cm.targetsDutyCycle {
		targets = append(targets, fmt.Sprintf("%s %d%% | ", target, dc))
	}
	sort.Strings(targets)
	for _, log := range targets {
		logString += log
	}

	fmt.Println(logString)
}
