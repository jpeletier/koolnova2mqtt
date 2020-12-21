package kn

import (
	"errors"
)

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

const HOLD_MODE_UNDERFLOOR_ONLY = "underfloor"
const HOLD_MODE_FAN_ONLY = "fan"
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

const HA_COMPONENT_SENSOR = "sensor"
const HA_COMPONENT_CLIMATE = "climate"

func FanMode2Str(fm FanMode) string {
	switch fm {
	case FAN_OFF:
		return "off"
	case FAN_LOW:
		return "low"
	case FAN_MED:
		return "medium"
	case FAN_HIGH:
		return "high"
	case FAN_AUTO:
		return "auto"
	default:
		return "unknown"
	}
}

func Str2FanMode(st string) (FanMode, error) {
	switch st {
	case "off":
		return FAN_OFF, nil
	case "low":
		return FAN_LOW, nil
	case "medium":
		return FAN_MED, nil
	case "high":
		return FAN_HIGH, nil
	case "auto":
		return FAN_AUTO, nil
	default:
		return FAN_OFF, errors.New("Unknown fan mode")
	}
}

func ApplyHvacMode(knMode KnMode, hvacMode string) KnMode {
	switch knMode {
	case MODE_AIR_COOLING:
		if hvacMode == HVAC_MODE_HEAT {
			return MODE_AIR_HEATING
		}
	case MODE_AIR_HEATING:
		if hvacMode == HVAC_MODE_COOL {
			return MODE_AIR_COOLING
		}
	case MODE_UNDERFLOOR_AIR_COOLING:
		if hvacMode == HVAC_MODE_HEAT {
			return MODE_UNDERFLOOR_AIR_HEATING
		}
	case MODE_UNDERFLOOR_AIR_HEATING, MODE_UNDERFLOOR_HEATING:
		if hvacMode == HVAC_MODE_COOL {
			return MODE_UNDERFLOOR_AIR_COOLING
		}
	}
	return knMode
}

func ApplyHoldMode(knMode KnMode, holdMode string) KnMode {
	cool := knMode == MODE_AIR_COOLING || knMode == MODE_UNDERFLOOR_AIR_COOLING
	switch holdMode {
	case HOLD_MODE_FAN_ONLY:
		if cool {
			return MODE_AIR_COOLING
		}
		return MODE_AIR_HEATING
	case HOLD_MODE_UNDERFLOOR_ONLY:
		if cool {
			return MODE_UNDERFLOOR_AIR_COOLING
		}
		return MODE_UNDERFLOOR_HEATING
	case HOLD_MODE_UNDERFLOOR_AND_FAN:
		if cool {
			return MODE_UNDERFLOOR_AIR_COOLING
		}
		return MODE_UNDERFLOOR_AIR_HEATING
	}
	return knMode
}
