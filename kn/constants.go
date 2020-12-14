package kn

import "koolnova2mqtt/bimap"

const NUM_ZONES = 16

const REG_PER_ZONE = 4
const REG_ENABLED = 1
const REG_MODE = 2
const REG_TARGET_TEMP = 3
const REG_CURRENT_TEMP = 4

const REG_AIRFLOW = 65
const REG_AC_TARGET_TEMP = 69
const REG_AC_TARGET_FAN_MODE = 73
const REG_SERIAL_CONFIG = 77
const REG_SLAVE_ID = 78
const REG_EFFICIENCY = 79
const REG_SYSTEM_ENABLED = 81
const REG_SYS_KN_MODE = 82

const FIRST_ZONE_REGISTER = REG_ENABLED
const TOTAL_ZONE_REGISTERS = NUM_ZONES * REG_PER_ZONE
const FIRST_SYS_REGISTER = REG_AIRFLOW
const TOTAL_SYS_REGISTERS = 18

type FanMode byte

const FAN_OFF FanMode = 0
const FAN_LOW FanMode = 1
const FAN_MED FanMode = 2
const FAN_HIGH FanMode = 3
const FAN_AUTO FanMode = 4

type KnMode byte

const MODE_AIR_COOLING KnMode = 0x01
const MODE_AIR_HEATING KnMode = 0x02
const MODE_UNDERFLOOR_HEATING KnMode = 0x04
const MODE_UNDERFLOOR_AIR_COOLING KnMode = 0x05
const MODE_UNDERFLOOR_AIR_HEATING KnMode = 0x06

var FanModes = bimap.New(map[interface{}]interface{}{
	"off":    FAN_OFF,
	"low":    FAN_LOW,
	"medium": FAN_MED,
	"high":   FAN_HIGH,
	"auto":   FAN_AUTO,
})

var KnModes = bimap.New(map[interface{}]interface{}{
	"air cooling":            MODE_AIR_COOLING,
	"air heating":            MODE_AIR_HEATING,
	"underfloor heating":     MODE_UNDERFLOOR_HEATING,
	"underfloor air cooling": MODE_UNDERFLOOR_AIR_COOLING,
	"underfloor air heating": MODE_UNDERFLOOR_AIR_HEATING,
})

const HOLD_MODE_UNDERFLOOR_ONLY = "underfloor only"
const HOLD_MODE_FAN_ONLY = "fan only"
const HOLD_MODE_UNDERFLOOR_AND_FAN = "underfloor and fan"

const HVAC_MODE_OFF = "off"
const HVAC_MODE_COOL = "cool"
const HVAC_MODE_HEAT = "heat"

type ACMachine int

const ACMachines = 4

const AC1 ACMachine = 1
const AC2 ACMachine = 2
const AC3 ACMachine = 3
const AC4 ACMachine = 4

func FanMode2Str(fm FanMode) string {
	st, ok := FanModes.GetInverse(fm)
	if !ok {
		st = "unknown"
	}
	return st.(string)
}

func KnMode2Str(hm KnMode) string {
	st, ok := KnModes.GetInverse(hm)
	if !ok {
		st = "unknown"
	}
	return st.(string)
}
