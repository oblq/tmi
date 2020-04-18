package commanderpro

import (
	"fmt"
	"io/ioutil"
	"os"
	"path/filepath"
	"sync"
	"time"

	"gopkg.in/yaml.v3"

	"github.com/google/gousb"
)

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
	Method string
	Arg    string
}

type Config struct {
	FanMode map[uint8]uint8 `yaml:"fan_mode"`

	// commanderpro, ipmi, cli
	// commanderpro: sensor_channel (int), ipmi: entityID, cli: custom_command
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

	configPath string
	configStat os.FileInfo

	config Config

	externalTempTicker *time.Ticker
	GetExternalTemp    func(method, arg string) (temp float64, err error)
}

func (cp *CommanderPro) Open() (err error) {
	// Initialize a new Context.
	cp.ctx = gousb.NewContext()

	// Open any device with a given VID/PID using a convenience function.
	cp.dev, err = cp.ctx.OpenDeviceWithVIDPID(vid, pid)
	if err != nil || cp.dev == nil {
		cp.Close()
		return fmt.Errorf("could not open a device: %v", err)
	}

	if err = cp.dev.SetAutoDetach(true); err != nil {
		cp.Close()
		return fmt.Errorf("unable to set autodetach on device: %v", err)
	}

	// Claim the default interface using a convenience function.
	// The default interface is always #0 alt #0 in the currently active
	// config.
	cp.intf, cp.intfDone, err = cp.dev.DefaultInterface()
	if err != nil {
		cp.Close()
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
		cp.Close()
		return fmt.Errorf("%s.InEndpoint(1): %v", cp.intf, err)
	}

	// And in the same interface open endpoint #2 for writing.
	cp.outEndpoint, err = cp.intf.OutEndpoint(2)
	if err != nil {
		cp.Close()
		return fmt.Errorf("%s.OutEndpoint(2): %v", cp.intf, err)
	}

	//go func() {
	//	for {
	//		// readBytes might be smaller than the buffer size. readBytes might be greater than zero even if err is not nil.
	//		buf := make([]byte, cp.inEndpoint.Desc.MaxPacketSize)
	//		readBytes, err := cp.inEndpoint.Read(buf)
	//		if err != nil {
	//			fmt.Printf("read error: %v\n", err)
	//		}
	//		if readBytes == 0 {
	//			fmt.Println("endpoint returned 0 bytes of data")
	//		}
	//
	//		fmt.Printf("read buffer: %#v\n", buf)
	//	}
	//}()

	return

	//return cp.LoadConfig()
}

func (cp *CommanderPro) Close() {
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

// LoadConfigThresholds will update ipmi fan thresholds.
// `sudo watch ipmitool sensor` to get the current settings.
func (cp *CommanderPro) LoadConfig() (err error) {
	if cp.configStat, err = os.Stat(cp.configPath); err != nil {
		return
	}

	configData, err := ioutil.ReadFile(cp.configPath)
	if err != nil {
		return err
	}
	err = yaml.Unmarshal(configData, &cp.config)
	if err != nil {
		return err
	}

	fmt.Println("commanderpro config updated")

	for ch, ledCount := range cp.config.LedCountPerCh {
		if err := cp.WriteLedCount(ch, ledCount); err != nil {
			return fmt.Errorf("error setting number of leds per channel: %d - %d -> %s", ch, ledCount, err.Error())
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
			return err
		}

		if groupConfig.LedMode == LedMode_Temperature {
			tempExtractor, ok := cp.config.ExternalTempExtractors[groupConfig.ExternalTemp]
			if !ok {
				return fmt.Errorf("no such external_temp: %s", groupConfig.ExternalTemp)
			}
			if externalTempExtractors[tempExtractor] == nil {
				externalTempExtractors[tempExtractor] = make([]uint8, 0)
			}
			externalTempExtractors[tempExtractor] = append(externalTempExtractors[tempExtractor], groupConfig.LedCh)
		}
	}

	cp.monitorExternalTempIfNeeded(externalTempExtractors)

	return nil
}

func (cp *CommanderPro) CheckConfig(configPath string) {
	cp.configPath = filepath.Join(configPath, "commanderpro.yaml")
	if configStat, err := os.Stat(cp.configPath); err != nil {
		fmt.Println("unable to stat config file:", err.Error())
	} else if cp.configStat == nil || configStat.Size() != cp.configStat.Size() ||
		configStat.ModTime() != cp.configStat.ModTime() {
		cp.configStat = configStat
		err = cp.LoadConfig()
		if err != nil {
			fmt.Println(err.Error())
		}
		return
	}
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

// ---------------------------------------------------------------------------------------------------------------------

// module interface implementation
func (cp *CommanderPro) Name() string {
	return "commanderpro"
}

// ---------------------------------------------------------------------------------------------------------------------

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
				temp, err := cp.GetExternalTemp(tExtractor.Method, tExtractor.Arg)
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
