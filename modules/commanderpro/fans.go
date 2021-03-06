package commanderpro

import "encoding/binary"

const (
	CMDGetFanMask           cmd = 0x20
	CMDGetFanRPM            cmd = 0x21 // CMDReadFanSpeed
	CMDGetFanFixedDutyCycle cmd = 0x22 // CMDReadFanPower
	CMDSetFanFixedDutyCycle cmd = 0x23 // CMDWriteFanPower
	CMDSetFanFixedRPM       cmd = 0x24 // CMDWriteFanSpeed
	CMDSetFanCustomCurve    cmd = 0x25 // CMDWriteFanCurve
	CMDWriteFanExternalTemp cmd = 0x26 // CMDWriteFanExternalTemp
	CMDFanForceThreePinMode cmd = 0x27 // CMDWriteFanForceThreePinMode
	CMDSetFanMode           cmd = 0x28 // CMDWriteFanDetectionType
	CMDGetFanMode           cmd = 0x29 // CMDReadFanDetectionType

	FanCh1 FanCh = 0x00
	FanCh2 FanCh = 0x01
	FanCh3 FanCh = 0x02
	FanCh4 FanCh = 0x03
	FanCh5 FanCh = 0x04
	FanCh6 FanCh = 0x05

	FanModeAutoDisconnected FanMode = 0x00
	FanMode3Pin             FanMode = 0x01
	FanMode4Pin             FanMode = 0x02
	FanModeUnknown          FanMode = 0x03
)

func (cp *CommanderPro) GetFanMask() (fan1, fan2, fan3, fan4, fan5, fan6 FanMode, err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDGetFanMask)

	resp, err := cp.cmd(cmd)
	if err != nil {
		return 0, 0, 0, 0, 0, 0, err
	}
	return FanMode(resp[1]),
		FanMode(resp[2]),
		FanMode(resp[3]),
		FanMode(resp[4]),
		FanMode(resp[5]),
		FanMode(resp[6]),
		nil
}

func (cp *CommanderPro) GetChannelRPM(fan FanCh) (rpm uint16, err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDGetFanRPM)
	cmd[1] = byte(fan)

	resp, err := cp.cmd(cmd)
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(resp[1:3]), nil
}

func (cp *CommanderPro) GetChannelDutyCycle(fan uint8) (dutyCycle uint8, err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDGetFanFixedDutyCycle)
	cmd[1] = fan

	resp, err := cp.cmd(cmd)
	return resp[1], err
}

// fanController interface implementation.
func (cp *CommanderPro) SetChannelDutyCycle(fan uint8, dutyCycle uint8) error {
	if dutyCycle == 0 {
		return cp.SetFanMode(FanCh(fan), FanModeUnknown)
	}

	if fanMode, err := cp.GetFanMode(FanCh(fan)); err != nil {
		return err
	} else if fanMode == FanModeUnknown {
		if err := cp.SetFanMode(FanCh(fan), FanModeAutoDisconnected); err != nil {
			return err
		}
	}

	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDSetFanFixedDutyCycle)
	cmd[1] = fan
	cmd[2] = dutyCycle

	_, err := cp.cmd(cmd)
	return err
}

func (cp *CommanderPro) SetChannelFixedRPM(fan FanCh, rpm uint16) error {
	if rpm == 0 {
		return cp.SetFanMode(fan, FanModeUnknown)
	}

	if fanMode, err := cp.GetFanMode(fan); err != nil {
		return err
	} else if fanMode == FanModeUnknown {
		if err := cp.SetFanMode(fan, FanModeAutoDisconnected); err != nil {
			return err
		}
	}

	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDSetFanFixedRPM)
	cmd[1] = byte(fan)
	binary.BigEndian.PutUint16(cmd[2:4], rpm)

	_, err := cp.cmd(cmd)
	return err
}

// Modes are listed in this order (shown with default values)
//"Default" (graph configuration request 0x25) 20 degC, 600 rpm ; 25 degC, 600 rpm ; 29 degC, 750 rpm ; 33 degC, 1000 rpm ; 37 degC, 1250 rpm ; 40 degC, 1500 rpm
//"Quiet" (graph configuration request 0x25) (same points as in "Default" mode above)
//"Balanced" (graph configuration request 0x25) 20 degC, 750 rpm ; 25 degC, 1000 rpm ; 29 degC, 1250 rpm ; 33 degC, 1500 rpm ; 37 degC, 1750 rpm ; 40 degC, 2000 rpm
//"Performance" (graph configuration request 0x25) 20 degC, 1000 rpm ; 25 degC, 1250 rpm ; 29 degC, 1500 rpm ; 33 degC, 1750 rpm ; 37 degC, 2000 rpm ; 40 degC, 2500 rpm
//"Custom" (graph configuration request 0x25) 20 degC, 600 rpm ; 30 degC, 600 rpm ; 40 degC, 750 rpm ; 50 degC, 1000 rpm ; 60 degC, 1250 rpm ; 70 degC, 1500 rpm
//"Fixed %" (percent configuration request 0x23) 40%
//"Fixed rpm" (rpm configuration request 0x24) 500
//"Max" (percent configuration request) (100%)
//Note that the default mode seems to be "Balanced".
func (cp *CommanderPro) SetChannelCustomCurve(fan FanCh, tempSensor TempSensor, temps [6]uint16, rpms [6]uint16) error {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDSetFanCustomCurve)
	cmd[1] = byte(fan)
	cmd[2] = byte(tempSensor)

	for i, t := range temps {
		idx := 3 + i*2
		t = t * 100
		binary.BigEndian.PutUint16(cmd[idx:idx+2], t)
	}

	for i, rpm := range rpms {
		idx := 15 + i*2
		binary.BigEndian.PutUint16(cmd[idx:idx+2], rpm)
	}

	_, err := cp.cmd(cmd)
	return err
}

// send to sensor? I don't think so... we send to a fan instead
func (cp *CommanderPro) WriteFanExternalTemp(sensorIndex uint8, temp uint16) error {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDWriteFanExternalTemp)
	cmd[1] = byte(sensorIndex)
	binary.BigEndian.PutUint16(cmd[2:4], temp*100)

	_, err := cp.cmd(cmd)
	return err
}

func (cp *CommanderPro) SetFanMode(fan FanCh, fanMode FanMode) error {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDSetFanMode)
	cmd[1] = 0x02
	cmd[2] = byte(fan)
	cmd[3] = byte(fanMode)

	_, err := cp.cmd(cmd)
	return err
}

func (cp *CommanderPro) GetFanMode(fan FanCh) (fanMode FanMode, err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDGetFanMode)
	cmd[1] = 0x01
	cmd[2] = byte(fan)

	resp, err := cp.cmd(cmd)

	if resp[2] == byte(fan) {
		return FanMode(resp[3]), err
	}
	return FanModeUnknown, err
}
