package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"plugin"
	"sort"
	"sync"
	"time"

	"github.com/google/gousb"
	"github.com/jessevdk/go-flags"
	"gopkg.in/yaml.v3"
)

// Options is Flags options: https://godoc.org/github.com/jessevdk/go-flags
type Options struct {
	// -c "my_config_path"
	ConfigPath string `short:"c" long:"config" description:"Configuration file path"`
}

// the main func is not run in plugin mode, only when built as normal Go program
func main() {
	var opts Options
	if _, err := flags.Parse(&opts); err != nil {
		fmt.Println(err.Error())
	}

	// set the default path for config
	if opts.ConfigPath == "" {
		opts.ConfigPath = "/opt/tmi"
	}

	cp, err := Open()
	if err != nil {
		panic(err)
	}

	cp.ReadConfig(opts.ConfigPath)

	ticker := time.NewTicker(time.Second * 2)
	for range ticker.C {
		temp1, err := cp.GetTempForSensor(TempSensor1)
		if err != nil {
			break
		}

		temp2, err := cp.GetTempForSensor(TempSensor2)
		if err != nil {
			break
		}

		temp3, err := cp.GetTempForSensor(TempSensor3)
		if err != nil {
			break
		}

		temp4, err := cp.GetTempForSensor(TempSensor4)
		if err != nil {
			break
		}

		dc1, err := cp.GetChannelRPM(FanCh1)
		if err != nil {
			break
		}

		dc2, err := cp.GetChannelRPM(FanCh2)
		if err != nil {
			break
		}

		dc3, err := cp.GetChannelRPM(FanCh3)
		if err != nil {
			break
		}

		dc4, err := cp.GetChannelRPM(FanCh4)
		if err != nil {
			break
		}

		dc5, err := cp.GetChannelRPM(FanCh5)
		if err != nil {
			break
		}

		dc6, err := cp.GetChannelRPM(FanCh6)
		if err != nil {
			break
		}

		fmt.Printf("Temps: 0: %.1f  1: %.1f  2: %.1f  3: %.1f     DutyCycles: 0: %d  1: %d  2: %d  3: %d  4: %d  5: %d \n",
			temp1, temp2, temp3, temp4, dc1, dc2, dc3, dc4, dc5, dc6)

		cp.ReadConfig(opts.ConfigPath)
	}
}

// list all devices:
//  go get -v github.com/google/gousb/lsusb
// lsusb
// Bus 001 Device 003: ID 1b1c:0c10 Corsair Commander PRO

//     {
//        .vendor_id = 0x1b1c,
//        .product_id = 0x0c10,
//        .device_id = 0xFF,
//        .name = "Commander PRO", /* Barbuda */
//        .read_endpoint = 0x01 | LIBUSB_ENDPOINT_IN,
//        .write_endpoint = 0x02 | LIBUSB_ENDPOINT_OUT,
//        .driver = &corsairlink_driver_commanderpro,
//        .lowlevel = &corsairlink_lowlevel_commanderpro,
//        .led_control_count = 2,
//        .fan_control_count = 6,
//        .pump_index = 0,
//    }

const (
	// Commander Pro vendor ID
	vid = gousb.ID(0x1b1c)

	// Commander Pro product ID
	pid = gousb.ID(0x0c10)
)

type cmd byte

type externalTempExtractor struct {
	Plugin string
	Arg    string

	// the last extracted temp
	lastTemp float64
}

type tempTarget struct {
	LedCh               LedCh
	LedOffset, LedCount uint8
}

type Config struct {
	FanMode map[FanCh]FanMode `yaml:"fan_mode"`

	// method: commanderpro, ipmi, cli
	// arg: {commanderpro: sensor_channel (int), ipmi: entityID, cli: custom_command}
	ExternalTempExtractors map[string]*externalTempExtractor `yaml:"external_temps"`

	FixedDutyCycles map[FanCh]uint8 `yaml:"fixed_duty_cycles"`

	CustomCurves FanCurves `yaml:"custom_curves"`

	LedCountPerCh map[LedCh]uint8 `yaml:"led_count_per_ch"`

	TempShiftTest struct {
		Enabled bool
		Ch      LedCh
		From    float64
		To      float64
	} `yaml:"temp_shift_test"`

	LedGroupConfigs map[string]LedGroupConfig `yaml:"led_group_configs"`
}

type CommanderPro struct {
	ctx *gousb.Context
	dev *gousb.Device
	//cfg      *gousb.Config
	intf     *gousb.Interface
	intfDone func()

	inEndpoint  *gousb.InEndpoint
	outEndpoint *gousb.OutEndpoint

	mutex sync.Mutex

	//configPath string
	configStat os.FileInfo

	config Config

	getDone                  chan bool
	externalTempGetterTicker *time.Ticker
	setDone                  chan bool
	externalTempSetterTicker *time.Ticker
	externalTempExtractors   map[string]TMITempExtractor
}

func (cp *CommanderPro) Open() (err error) {
	// Initialize a new Context.
	cp.ctx = gousb.NewContext()

	// Open any device with a given VID/PID using a convenience function.
	cp.dev, err = cp.ctx.OpenDeviceWithVIDPID(vid, pid)
	if err != nil || cp.dev == nil {
		cp.ShutDown()
		return fmt.Errorf("could not open a device: %v", err)
	}

	if err = cp.dev.SetAutoDetach(true); err != nil {
		cp.ShutDown()
		return fmt.Errorf("unable to set autodetach on device: %v", err)
	}

	// Claim the default interface using a convenience function.
	// The default interface is always #0 alt #0 in the currently active config.
	cp.intf, cp.intfDone, err = cp.dev.DefaultInterface()
	if err != nil {
		cp.ShutDown()
		return fmt.Errorf("%s.DefaultInterface() error: %v", cp.dev, err)
	}

	//// Switch the configuration to #2.
	//cp.cfg, err = cp.dev.Config(1)
	//if err != nil {
	//	log.Fatalf("%s.Config(2): %v", cp.dev, err)
	//}
	//
	//// In the config #2, claim interface #3 with alt setting #0.
	//cp.intf, err = cp.cfg.Interface(0, 0)
	//if err != nil {
	//	log.Fatalf("%s.Interface(3, 0): %v", cp.cfg, err)
	//}

	// Open an OUT endpoint.
	cp.inEndpoint, err = cp.intf.InEndpoint(1)
	if err != nil {
		cp.ShutDown()
		return fmt.Errorf("%s.InEndpoint(1): %v", cp.intf, err)
	}

	// And in the same interface open endpoint #2 for writing.
	cp.outEndpoint, err = cp.intf.OutEndpoint(2)
	if err != nil {
		cp.ShutDown()
		return fmt.Errorf("%s.OutEndpoint(2): %v", cp.intf, err)
	}

	return
}

func (cp *CommanderPro) cmd(cmd []byte) (response []byte, err error) {
	cp.mutex.Lock()
	defer cp.mutex.Unlock()

	// Write data to the USB device.
	numBytes, err := cp.outEndpoint.Write(cmd)
	if numBytes != len(cmd) {
		return nil, fmt.Errorf("%s.Write(): only %d bytes written, returned error is %v", cp.outEndpoint, numBytes, err)
	}

	// readBytes might be smaller than the buffer size. readBytes might be greater than zero even if err is not nil.
	buf := make([]byte, cp.inEndpoint.Desc.MaxPacketSize)
	readBytes, err := cp.inEndpoint.Read(buf)
	if err != nil {
		return buf, fmt.Errorf("read error: %v", err)
	}
	if readBytes == 0 {
		return buf, fmt.Errorf("endpoint returned 0 bytes of data")
	}

	return buf, nil
}

func Open() (cp *CommanderPro, err error) {
	cp = &CommanderPro{}
	err = cp.Open()
	return
}

// pluggableModule interface implementation ----------------------------------------------------------------------------

func Plugin() interface{} {
	cp, err := Open()
	if err != nil {
		panic(err)
	}

	return cp
}

// module interface implementation
func (cp *CommanderPro) Name() string {
	return "commanderpro"
}

func (cp *CommanderPro) ReadConfig(configPath string) {

	path := filepath.Join(configPath, "commanderpro.yaml")

	if configStat, err := os.Stat(path); err != nil {

		fmt.Println("unable to stat config file:", err.Error())

	} else if cp.configStat == nil ||
		configStat.Size() != cp.configStat.Size() ||
		configStat.ModTime() != cp.configStat.ModTime() {

		cp.configStat = configStat

		configData, err := ioutil.ReadFile(path)
		if err != nil {
			fmt.Println(err.Error())
			return
		}
		err = yaml.Unmarshal(configData, &cp.config)
		if err != nil {
			fmt.Println(err.Error())
			return
		}

		fmt.Println("commanderpro config updated")

		if len(cp.config.FixedDutyCycles) > 0 {
			for fanCh, dutyCycle := range cp.config.FixedDutyCycles {
				if err := cp.SetChannelDutyCycle(fanCh, dutyCycle); err != nil {
					fmt.Println(err.Error())
					return
				}
			}
		}

		if err = cp.SetCustomCurve(cp.config.CustomCurves); err != nil {
			fmt.Println(err.Error())
			return
		}

		for ch, ledCount := range cp.config.LedCountPerCh {
			if err := cp.WriteLedCount(ch, ledCount); err != nil {
				detailedErr := fmt.Errorf("error setting number of leds per channel: %d - %d -> %s", ch, ledCount, err.Error())
				fmt.Println(detailedErr.Error())
				return
			}
		}

		externalTempExtractors := make(map[*externalTempExtractor][]tempTarget)

		if cp.config.TempShiftTest.Enabled {
			go cp.tempShift(cp.config.TempShiftTest.Ch, cp.config.TempShiftTest.From, cp.config.TempShiftTest.To)
		}

		if err = cp.ClearGroup(LedCh1); err != nil {
			return
		}

		if err = cp.ClearGroup(LedCh2); err != nil {
			return
		}

		for _, groupConfig := range cp.config.LedGroupConfigs {
			err = cp.WriteLedGroupSet(
				groupConfig.LedCh,
				groupConfig.LedOffset,
				groupConfig.LedCount,
				groupConfig.LedMode,
				groupConfig.LedSpeed,
				groupConfig.LedDirection,
				groupConfig.LedStyle,
				groupConfig.Color1,
				groupConfig.Color2,
				groupConfig.Color3,
				groupConfig.Temp1,
				groupConfig.Temp2,
				groupConfig.Temp3)
			if err != nil {
				fmt.Println(err.Error())
				return
			}

			if groupConfig.LedMode == LedMode_Temperature {
				cp.loadPlugins(configPath)

				tempExtractor, ok := cp.config.ExternalTempExtractors[groupConfig.ExternalTemp]
				if !ok {
					fmt.Printf("no such external_temp: %s \n", groupConfig.ExternalTemp)

				}
				if externalTempExtractors[tempExtractor] == nil {
					externalTempExtractors[tempExtractor] = make([]tempTarget, 0)
				}
				tempTarget := tempTarget{LedCh: groupConfig.LedCh, LedOffset: groupConfig.LedOffset, LedCount: groupConfig.LedCount}
				externalTempExtractors[tempExtractor] = append(externalTempExtractors[tempExtractor], tempTarget)
			}
		}

		cp.monitorExternalTempIfNeeded(externalTempExtractors)
	}
}

func (cp *CommanderPro) loadPlugins(configPath string) {
	cp.externalTempExtractors = make(map[string]TMITempExtractor, len(cp.config.ExternalTempExtractors))
	for _, ete := range cp.config.ExternalTempExtractors {
		if ete.Plugin == cp.Name() {
			cp.externalTempExtractors[ete.Plugin] = cp
			continue
		}

		p, err := plugin.Open(filepath.Join(configPath, fmt.Sprintf("%s.so", ete.Plugin)))
		if err != nil {
			panic(err)
		}

		Plugin, err := p.Lookup("Plugin")
		if err != nil {
			panic(err)
		}

		i := Plugin.(TMIPluginGetter)()

		if p, ok := i.(TMITempExtractor); ok {
			p.ReadConfig(configPath)
			cp.externalTempExtractors[p.Name()] = p
		}
	}
}

func (cp *CommanderPro) ShutDown() {
	if cp.ctx != nil {
		_ = cp.ctx.Close()
	}
	if cp.dev != nil {
		_ = cp.dev.Close()
	}
	//if cp.cfg != nil {
	//	_ = cp.cfg.Close()
	//}
	if cp.intfDone != nil {
		cp.intfDone()
	}
	if cp.intf != nil {
		cp.intf.Close()
	}
}

func (cp *CommanderPro) monitorExternalTempIfNeeded(tempExtractors map[*externalTempExtractor][]tempTarget) {
	if cp.config.TempShiftTest.Enabled {
		return
	}

	if cp.getDone != nil {
		close(cp.getDone)
	}
	if cp.setDone != nil {
		close(cp.setDone)
	}

	if len(tempExtractors) == 0 {
		if cp.externalTempGetterTicker != nil {
			cp.externalTempGetterTicker.Stop()
		}
		if cp.externalTempSetterTicker != nil {
			cp.externalTempSetterTicker.Stop()
		}

		return
	}

	for _, channels := range tempExtractors {
		sort.Slice(channels, func(i, j int) bool {
			return channels[i].LedOffset < channels[j].LedOffset
		})
	}

	getTemps := func() {
		for tExtractor := range tempExtractors {
			temp, err := cp.externalTempExtractors[tExtractor.Plugin].GetTemp(tExtractor.Arg)
			if err != nil {
				fmt.Println("unable to extract temp for led channel:", err.Error())
				continue
			}
			tExtractor.lastTemp = temp
		}
	}

	setTemps := func() {
		//fmt.Println()
		for tExtractor, channels := range tempExtractors {
			for _, target := range channels {
				//fmt.Println(target.LedCh, target.LedOffset, tExtractor.lastTemp)
				if err := cp.WriteLedExternalTemp(target.LedCh, target.LedOffset, tExtractor.lastTemp); err != nil {
					fmt.Println("unable to send temp to led channel:", err.Error())
				}
			}
		}
	}

	go func() {
		getTemps()
		setTemps()
	}()

	cp.getDone = make(chan bool, 1)
	cp.externalTempGetterTicker = time.NewTicker(time.Second * 3)
	go func() {
		for {
			select {
			case <-cp.externalTempGetterTicker.C:
				getTemps()
			case <-cp.getDone:
				cp.getDone = nil
				return
			}
		}
	}()

	cp.setDone = make(chan bool, 1)
	cp.externalTempSetterTicker = time.NewTicker(time.Second * 1)
	go func() {
		for {
			select {
			case <-cp.externalTempSetterTicker.C:
				setTemps()
			case <-cp.setDone:
				cp.setDone = nil
				return
			}
		}
	}()
}
