package zonewatcher

import "koolnova2mqtt/bimap"

const REG_PER_ZONE = 4
const REG_ENABLED = 1
const REG_MODE = 2
const REG_TARGET_TEMP = 3
const REG_CURRENT_TEMP = 4

type FanMode byte

const FAN_OFF FanMode = 0x00
const FAN_LOW FanMode = 0x10
const FAN_MED FanMode = 0x20
const FAN_HIGH FanMode = 0x30
const FAN_AUTO FanMode = 0x40

type HvacMode byte

const MODE_AIR_COOLING HvacMode = 0x01
const MODE_AIR_HEATING HvacMode = 0x02
const MODE_UNDERFLOOR_HEATING HvacMode = 0x04
const MODE_UNDERFLOOR_AIR_COOLING HvacMode = 0x05
const MODE_UNDERFLOOR_AIR_HEATING HvacMode = 0x06

var FanModes = bimap.New(map[interface{}]interface{}{
	"off":    FAN_OFF,
	"low":    FAN_LOW,
	"medium": FAN_MED,
	"high":   FAN_HIGH,
	"auto":   FAN_AUTO,
})

var HvacModes = bimap.New(map[interface{}]interface{}{
	"air cooling":            MODE_AIR_COOLING,
	"air heating":            MODE_AIR_HEATING,
	"underfloor heating":     MODE_UNDERFLOOR_HEATING,
	"underfloor air cooling": MODE_UNDERFLOOR_AIR_COOLING,
	"underfloor air heating": MODE_UNDERFLOOR_AIR_HEATING,
})
