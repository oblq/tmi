package main

import (
	"encoding/binary"
	"time"
)

const (
	CMDReadLedStripMask     cmd = 0x30
	CMDWriteLedRgbValue     cmd = 0x31
	CMDWriteLedColorValues  cmd = 0x32
	CMDWriteLedTrigger      cmd = 0x33 // transmitted as the last command after a sequence of 0x3? commands, save changes
	CMDWriteLedClear        cmd = 0x34
	CMDWriteLedGroupSet     cmd = 0x35 // led config, transmitted for each LED strip/fan enabled, for each LED channel
	CMDWriteLedExternalTemp cmd = 0x36
	CMDWriteLedGroupsClear  cmd = 0x37

	// port = channel
	// int corsairlink_commanderpro_set_port_state(
	//    struct corsair_device_info* dev,
	//    struct libusb_device_handle* handle,
	//    struct led_control *ctrl )
	//{
	//    int rr;
	//    uint8_t response[16];
	//    uint8_t commands[64];
	//    memset( response, 0, sizeof( response ) );
	//    memset( commands, 0, sizeof( commands ) );
	//
	//    commands[0] = 0x38;
	//    commands[1] = ctrl->channel;
	//    commands[2] = 0x01;
	//
	//    rr = dev->lowlevel->write( handle, dev->write_endpoint, commands, 64 );
	//    rr = dev->lowlevel->read( handle, dev->read_endpoint, response, 16 );
	//
	//    dump_packet( commands, sizeof( commands ) );
	//    dump_packet( response, sizeof( response ) );
	//    return rr;
	//}
	//
	//
	// /**
	// * The mode of an LEDChannel. The mode describes how the LED lighting is done.
	// *
	// * @see LEDController#setLEDMode()
	// */
	//enum class ChannelMode : byte {
	//	/** No lighting is active for the channel. The LEDs will not be updated. */
	//	Disabled = 0x00,
	//	/** The Hardware Playback uses lighting effects defined by LEDGroups and LEDController renders the effects themself.
	//	   This mode works even without an USB connection.  */
	//	HardwarePlayback = 0x01,
	//	/** All lighting effects are rendered by iCUE and only the RGB values are transferred via USB to the device. This
	//	   requires an USB connection. */
	//	SoftwarePlayback = 0x02
	//};
	CMDWriteLedMode       cmd = 0x38
	CMDWriteLedBrightness cmd = 0x39
	CMDWriteLedCount      cmd = 0x3a
	CMDWriteLedPortType   cmd = 0x3b // protocol

	LedCh1 = 0x00
	LedCh2 = 0x01

	LedMode_RainbowWave = 0x00
	LedMode_ColorShift  = 0x01
	LedMode_ColorPulse  = 0x02
	LedMode_ColorWave   = 0x03
	LedMode_Static      = 0x04
	LedMode_Temperature = 0x05
	LedMode_Visor       = 0x06
	LedMode_Marquee     = 0x07
	LedMode_Blink       = 0x08
	LedMode_Sequential  = 0x09
	LedMode_Rainbow     = 0x0A

	LedSpeedHigh   = 0x00
	LedSpeedMedium = 0x01
	LedSpeedLow    = 0x02

	LedDirection_Backward = 0x00
	LedDirection_Forward  = 0x01

	LedStyle_Alternating = 0x00
	LedStyle_RandomColor = 0x01
)

var (
	WarningColor = Color{
		R: 0xFF,
		G: 0x00,
		B: 0x00,
	}

	DefaultColor = Color{
		R: 0xFF,
		G: 0xFF,
		B: 0x00,
	}

// #define INIT_RAINBOW_LED(xx)
//  xx[0].red = 0xff;
//  xx[0].green = 0x00;
//  xx[0].blue = 0x00;
//  xx[1].red = 0xff;
//  xx[1].green = 0x80;
//  xx[1].blue = 0x00;
//  xx[2].red = 0xff;
//  xx[2].green = 0xff;
//  xx[2].blue = 0x00;
//  xx[3].red = 0x00;
//  xx[3].green = 0xff;
//  xx[3].blue = 0x00;
//  xx[4].red = 0x00;
//  xx[4].green = 0x00;
//  xx[4].blue = 0xff;
//  xx[5].red = 0x4b;
//  xx[5].green = 0x00;
//  xx[5].blue = 0x82;
//  xx[6].red = 0x7f;
//  xx[6].green = 0x00;
//  xx[6].blue = 0xff;
)

func (cp *CommanderPro) save() (err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDWriteLedTrigger)
	cmd[1] = 0xFF

	_, err = cp.cmd(cmd)
	return
}

func (cp *CommanderPro) Clear(ch uint8) (err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDWriteLedClear)
	cmd[1] = ch

	_, err = cp.cmd(cmd)
	return
}

func (cp *CommanderPro) WriteLedGroupSet(ledCh, offset, len, ledMode, ledSpeed, ledDirection, ledStyle uint8,
	color1, color2, color3 Color,
	temp1, temp2, temp3 float64) (err error) {

	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDWriteLedGroupSet)
	cmd[1] = ledCh
	cmd[2] = offset // led index
	cmd[3] = len    // leds count
	cmd[4] = ledMode
	cmd[5] = ledSpeed
	cmd[6] = ledDirection
	cmd[7] = ledStyle
	cmd[8] = 0xFF

	if ledStyle != LedStyle_RandomColor {
		cmd[9] = color1.R
		cmd[10] = color1.G
		cmd[11] = color1.B

		cmd[12] = color2.R
		cmd[13] = color2.G
		cmd[14] = color2.B

		cmd[15] = color3.R
		cmd[16] = color3.G
		cmd[17] = color3.B
	}

	if ledMode == LedMode_Temperature {
		binary.BigEndian.PutUint16(cmd[20:22], uint16(temp2*100))
		binary.BigEndian.PutUint16(cmd[22:24], uint16(temp3*100))
		binary.BigEndian.PutUint16(cmd[18:20], uint16(temp1*100))
	}

	_, err = cp.cmd(cmd)
	if err != nil {
		return
	}
	return cp.save()
}

func (cp *CommanderPro) ClearGroup(ch uint8) (err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDWriteLedGroupsClear)
	cmd[1] = ch

	_, err = cp.cmd(cmd)
	return
}

// Transmitted regularly by the LINK software to notify temperature to each LED channel.
// These are transmitted for each LED channel, one for all strips/fans configured on that channel,
// if "Temperature" mode is enabled AND "Group" is set accordingly.
//    commands[0] = 0x36;
//    commands[1] = ctrl->channel;
//    commands[2] = ctrl->channel;
//    commands[3] = 0x0A;
//    commands[4] = 0x28;
func (cp *CommanderPro) WriteLedExternalTemp(ledCh uint8, temp float64) (err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDWriteLedExternalTemp)
	cmd[1] = ledCh
	cmd[2] = 0x00
	binary.BigEndian.PutUint16(cmd[3:5], uint16(temp*100))

	_, err = cp.cmd(cmd)
	if err != nil {
		return
	}
	return cp.save()
}

// brightness is 0-100
func (cp *CommanderPro) WriteLedBrightness(ledCh uint8, brightness uint8) (err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDWriteLedBrightness)
	cmd[1] = ledCh
	cmd[2] = brightness

	_, err = cp.cmd(cmd)
	if err != nil {
		return
	}
	return cp.save()
}

func (cp *CommanderPro) WriteLedCount(ledCh uint8, count uint8) (err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDWriteLedCount)
	cmd[1] = ledCh
	cmd[2] = count

	_, err = cp.cmd(cmd)
	if err != nil {
		return
	}
	return cp.save()
}

// ---------------------------------------------------------------------------------------------------------------------

// Simulate a temperature shift from min to max and vice versa.
func (cp *CommanderPro) tempShift(ch uint8, from, to float64) {
	i := from

	if from < to {
		for i <= to {
			_ = cp.WriteLedExternalTemp(ch, i)
			time.Sleep(time.Millisecond * 500)
			i += 2
		}
	}

	if from > to {
		for i >= to {
			_ = cp.WriteLedExternalTemp(ch, i)
			time.Sleep(time.Millisecond * 500)
			i -= 2
		}
	}

	if cp.config.TempShiftTest.Enabled {
		cp.tempShift(ch, to, from)
	}
}
