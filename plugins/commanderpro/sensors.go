package main

import (
	"encoding/binary"
	"strconv"
)

const (
	CMDConnectedSensors cmd = 0x10 // CMDReadTemperatureMask
	CMDGetTemp          cmd = 0x11 // CMDReadTemperatureValue
	CMDGetVoltage       cmd = 0x12 // CMDReadVoltageValue

	TempSensorExternal TempSensor = 0xFF // ...then send temperature regularly to the Commander Pro with 0x26 cmd
	TempSensor1        TempSensor = 0x00 // 1
	TempSensor2        TempSensor = 0x01 // 2
	TempSensor3        TempSensor = 0x02 // 3
	TempSensor4        TempSensor = 0x03 // 4
)

// TMI tempExtractor interface implementation --------------------------------------------------------------------------

func (cp *CommanderPro) GetTemp(sensor string) (temp float64, err error) {
	var sNR uint64
	sNR, err = strconv.ParseUint(sensor, 16, 8)
	if err == nil {
		return
	}

	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDGetTemp)
	cmd[1] = byte(sNR)

	resp, err := cp.cmd(cmd)
	if err != nil {
		return 0, err
	}
	return float64(binary.BigEndian.Uint16(resp[1:3])) / 100, nil
}

// ---------------------------------------------------------------------------------------------------------------------

func (cp *CommanderPro) GetTempForSensor(sensor TempSensor) (temp float64, err error) {
	cmd := make([]byte, cp.outEndpoint.Desc.MaxPacketSize)
	cmd[0] = byte(CMDGetTemp)
	cmd[1] = byte(sensor)

	resp, err := cp.cmd(cmd)
	if err != nil {
		return 0, err
	}
	return float64(binary.BigEndian.Uint16(resp[1:3])) / 100, nil
}
