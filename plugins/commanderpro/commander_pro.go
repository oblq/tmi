package main

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"plugin"
	"sync"
	"time"

	"github.com/google/gousb"
	"gopkg.in/yaml.v3"
)

func main() {

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
type FanCh byte
type FanMode byte
type TempSensor byte

type Color struct {
	R uint8
	G uint8
	B uint8
}

type externalTempExtractor struct {
	Plugin string
	Arg    string
}

type Config struct {
	FanMode map[uint8]uint8 `yaml:"fan_mode"`

	// method: commanderpro, ipmi, cli
	// arg: {commanderpro: sensor_channel (int), ipmi: entityID, cli: custom_command}
	ExternalTempExtractors map[string]externalTempExtractor `yaml:"external_temps"`

	LedCountPerCh map[uint8]uint8 `yaml:"led_count_per_ch"`

	TempShiftTest struct {
		Enabled bool
		Ch      uint8
		From    float64
		To      float64
	} `yaml:"temp_shift_test"`

	LedGroupConfigs map[string]struct {
		LedCh, LedOffset, LedCount, LedMode, LedSpeed, LedDirection, LedStyle uint8
		Color1, Color2, Color3                                                Color
		Temp1, Temp2, Temp3                                                   float64
		ExternalTemp                                                          string
	} `yaml:"led_group_configs"`
}

func Open() (cp *CommanderPro, err error) {
	cp = &CommanderPro{}
	err = cp.Open()
	return
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

	externalTempTicker     *time.Ticker
	externalTempExtractors map[string]TMITempExtractor
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
	// The default interface is always #0 alt #0 in the currently active
	// config.
	cp.intf, cp.intfDone, err = cp.dev.DefaultInterface()
	if err != nil {
		cp.ShutDown()
		return fmt.Errorf("%s.DefaultInterface(): %v", cp.dev, err)
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

func (cp *CommanderPro) monitorExternalTempIfNeeded(tempExtractors map[externalTempExtractor][]uint8) {
	if cp.config.TempShiftTest.Enabled {
		return
	}

	if len(tempExtractors) == 0 {
		if cp.externalTempTicker != nil {
			cp.externalTempTicker.Stop()
		}
		return
	}

	cp.externalTempTicker = time.NewTicker(time.Second * time.Duration(4))
	go func() {
		for range cp.externalTempTicker.C {
			for tExtractor, channels := range tempExtractors {
				temp, err := cp.externalTempExtractors[tExtractor.Plugin].GetTemp(tExtractor.Arg)
				if err != nil {
					fmt.Println("unable to extract temp for led channel:", err.Error())
					continue
				}
				for _, ch := range channels {
					if err := cp.WriteLedExternalTemp(ch, temp); err != nil {
						fmt.Println("unable to send temp to led channel:", err.Error())
					}
				}
			}
		}
	}()
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

		for ch, ledCount := range cp.config.LedCountPerCh {
			if err := cp.WriteLedCount(ch, ledCount); err != nil {
				detailedErr := fmt.Errorf("error setting number of leds per channel: %d - %d -> %s", ch, ledCount, err.Error())
				fmt.Println(detailedErr.Error())
				return
			}
		}

		externalTempExtractors := make(map[externalTempExtractor][]uint8)

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
				cp.loadPlugins(path)

				tempExtractor, ok := cp.config.ExternalTempExtractors[groupConfig.ExternalTemp]
				if !ok {
					fmt.Printf("no such external_temp: %s \n", groupConfig.ExternalTemp)

				}
				if externalTempExtractors[tempExtractor] == nil {
					externalTempExtractors[tempExtractor] = make([]uint8, 0)
				}
				externalTempExtractors[tempExtractor] = append(externalTempExtractors[tempExtractor], groupConfig.LedCh)
			}
		}

		cp.monitorExternalTempIfNeeded(externalTempExtractors)
	}
}

func (cp *CommanderPro) loadPlugins(configPath string) {
	for _, ete := range cp.config.ExternalTempExtractors {
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
		cp.ctx.Close()
	}
	if cp.dev != nil {
		cp.dev.Close()
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
