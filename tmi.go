package main

import (
	"errors"
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"plugin"
	"sort"
	"strconv"
	"strings"
	"sync"
	"time"

	"gopkg.in/yaml.v3"
)

type TMIConfig struct {
	// Plugins is an array of plugins names to load at startup
	Plugins []string `yaml:"plugins"`

	// checkInterval is the time between checks, in seconds.
	CheckInterval int `yaml:"check_interval"`

	// a map containing arbitrary names associated with a TMIFanController:channel couple.
	// eg.: `pump: ipmi.0` or `side: commanderpro.2`
	TargetsMap map[string]string `yaml:"targets_map"`

	// Controllers is an array of controllers.
	// A controller consist in a struct describing the
	// method to be used to obtain the temperature
	// and the temp/duty-cycle mapping to be used for
	// one or most specified targets.
	Controllers []*controller `yaml:"controllers"`
}

type TMI struct {
	mutex   sync.Mutex
	ticker  *time.Ticker
	running bool

	configPath string
	configStat os.FileInfo
	config     *TMIConfig

	// always use pointers
	plugins        map[string]TMIPlugin
	tempExtractors map[string]TMITempExtractor
	fanControllers map[string]TMIFanController

	// prepared target list with parsed TMIFanController and channel
	targets map[string]*target
}

func New(configPath string) (cm *TMI, err error) {
	cm = &TMI{
		configPath:     configPath,
		plugins:        make(map[string]TMIPlugin),
		tempExtractors: make(map[string]TMITempExtractor),
		fanControllers: make(map[string]TMIFanController),
		targets:        make(map[string]*target),
	}

	err = cm.loadConfigAndStart()

	return
}

// loadConfigAndStart hot-reload the program configuration.
func (tmi *TMI) loadConfigAndStart() (err error) {
	tmi.mutex.Lock()

	tmi.StopMonitoring()

	configPath := filepath.Join(tmi.configPath, "tmi.yaml")
	if tmi.configStat, err = os.Stat(configPath); err != nil {
		return
	}

	config, err := ioutil.ReadFile(configPath)
	if err != nil {
		return
	}
	err = yaml.Unmarshal(config, &tmi.config)
	if err != nil {
		return
	}

	tmi.loadPlugins()

	err = tmi.parseTargetsMap()
	if err != nil {
		return
	}

	tmi.mutex.Unlock()

	for _, fc := range tmi.plugins {
		fc.ReadConfig(tmi.configPath)
	}

	tmi.StartMonitoring()

	return
}

// loadPlugins load all the needed plugins
func (tmi *TMI) loadPlugins() {
	for _, pName := range tmi.config.Plugins {
		p, err := plugin.Open(filepath.Join(tmi.configPath, fmt.Sprintf("%s.so", pName)))
		if err != nil {
			panic(err)
		}

		Plugin, err := p.Lookup("Plugin")
		if err != nil {
			panic(err)
		}

		i := Plugin.(TMIPluginGetter)()

		if p, ok := i.(TMIPlugin); ok {
			tmi.plugins[p.Name()] = p

			if te, ok := i.(TMITempExtractor); ok {
				tmi.tempExtractors[te.Name()] = te
			}

			if fc, ok := i.(TMIFanController); ok {
				tmi.fanControllers[fc.Name()] = fc
			}
		}
	}
}

func (tmi *TMI) hasPlugin(moduleName string) bool {
	if _, ok := tmi.plugins[moduleName]; ok {
		return true
	}
	return false
}

// parseTargetsMap parse the list of targets by name,
// extracting the plugin and the corresponding channel.
func (tmi *TMI) parseTargetsMap() error {
	for targetName, pluginChannel := range tmi.config.TargetsMap {
		coupleArr := strings.SplitN(pluginChannel, ".", 2)
		if len(coupleArr) < 2 {
			return errors.New("target_map value must be a string containing the target and one of its channels separated by a dot. eg.: `ipmi.0` ")
		}
		pluginName := coupleArr[0]
		channel := coupleArr[1]
		fanController, ok := tmi.fanControllers[pluginName]
		if !ok {
			return fmt.Errorf("no such target %s", pluginName)
		}
		fanControllerChannel, err := strconv.Atoi(channel)
		if err != nil {
			return fmt.Errorf("target channel for %s is not a valid integer", pluginName)
		}
		tmi.targets[targetName] = &target{
			fanController: fanController,
			channel:       uint8(fanControllerChannel),
		}
	}

	return nil
}

// StartMonitoring start the monitoring daemon,
// checking temps and duty-cycles.
func (tmi *TMI) StartMonitoring() {
	if tmi.running {
		return
	}

	tmi.running = true
	tmi.checkTemperatures()

	if tmi.ticker != nil {
		tmi.ticker.Stop()
	}
	tmi.ticker = time.NewTicker(time.Second * time.Duration(tmi.config.CheckInterval))
	go func() {
		for range tmi.ticker.C {
			tmi.checkTemperatures()
			tmi.checkConfig()
		}
	}()
}

// StopMonitoring stop the daemon.
func (tmi *TMI) StopMonitoring() {
	if tmi.ticker != nil {
		tmi.ticker.Stop()
	}
	tmi.running = false
}

// checkTemperatures is periodically called from the ticker to check the
// temperature for any of the TMI controllers.
func (tmi *TMI) checkTemperatures() {
	logString := "	| "

	tmi.mutex.Lock()

	// grab the greater values divided by zone first
	tempTargetsDutyCycles := make(map[string]uint8)
	for _, controller := range tmi.config.Controllers {
		tg, ok := tmi.tempExtractors[controller.Temp.Plugin]
		if !ok {
			fmt.Println("no such temp method: " + controller.Temp.Plugin)
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
		t, ok := tmi.targets[target]
		if !ok {
			fmt.Println("no such target: " + target)
			continue
		}

		if dc == 0 || tmi.targets[target].dutyCycle != dc {
			tmi.targets[target].dutyCycle = dc

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

	tmi.mutex.Unlock()

	logString += "	->	| "

	targets := make([]string, 0)
	for targetName, target := range tmi.targets {
		targets = append(targets, fmt.Sprintf("%s %d%% | ", targetName, target.dutyCycle))
	}
	sort.Strings(targets)
	for _, log := range targets {
		logString += log
	}

	fmt.Println(logString)
}

// checkConfig is periodically called from the ticker to check the
// TMI configuration.
func (tmi *TMI) checkConfig() {
	configPath := filepath.Join(tmi.configPath, "tmi.yaml")
	if configStat, err := os.Stat(configPath); err != nil {
		fmt.Println("unable to stat config file:", err.Error())
	} else if tmi.configStat == nil ||
		configStat.Size() != tmi.configStat.Size() || configStat.ModTime() != tmi.configStat.ModTime() {
		tmi.configStat = configStat

		if err := tmi.loadConfigAndStart(); err != nil {
			fmt.Println(err.Error())
		}

		fmt.Println("config updated")

		return
	}

	for _, p := range tmi.plugins {
		p.ReadConfig(tmi.configPath)
	}
}
