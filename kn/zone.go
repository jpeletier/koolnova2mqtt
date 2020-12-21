package kn

import (
	"math"

	average "github.com/RobinUS2/golang-moving-average"
)

type Watcher interface {
	ReadRegister(address uint16) (value uint16)
	WriteRegister(address uint16, value uint16) error
	RegisterCallback(address uint16, callback func(address uint16))
}

type ZoneConfig struct {
	ZoneNumber int
	Watcher    Watcher
}

// Zone is a driver to interact with climate zones
type Zone struct {
	ZoneConfig
	OnEnabledChange     func()
	OnCurrentTempChange func(newTemp float32)
	OnTargetTempChange  func(newTemp float32)
	OnFanModeChange     func(newMode FanMode)
	OnKnModeChange      func(newMode KnMode)
	lastTemp            float32
	temp                *average.MovingAverage
}

// newZone creates a new climate zone with the supplied configuration
// Invokes callbacks when specific registers change
// reads registers to return current state
// sets the appropriate registers when set* methods are invoked
func newZone(config *ZoneConfig) *Zone {
	z := &Zone{
		ZoneConfig: *config,
		temp:       average.New(300),
	}
	z.registerCallback(REG_ENABLED, func() {
		if z.OnEnabledChange != nil {
			z.OnEnabledChange()
		}
	})
	z.registerCallback(REG_TARGET_TEMP, func() {
		if z.OnTargetTempChange == nil {
			return
		}

		temp := z.getTargetTemperature()
		z.OnTargetTempChange(temp)
	})
	z.registerCallback(REG_MODE, func() {
		fanMode := z.getFanMode()
		hvacMode := z.getKnMode()
		if z.OnFanModeChange != nil {
			z.OnFanModeChange(fanMode)
		}
		if z.OnKnModeChange != nil {
			z.OnKnModeChange(hvacMode)
		}
	})
	return z
}

func (z *Zone) registerCallback(num int, f func()) {
	z.Watcher.RegisterCallback(uint16((z.ZoneNumber-1)*REG_PER_ZONE+num), func(address uint16) {
		f()
	})
}

func (z *Zone) readRegister(num int) uint16 {
	return z.Watcher.ReadRegister(uint16((z.ZoneNumber-1)*REG_PER_ZONE + num))
}

func (z *Zone) writeRegister(num int, value uint16) error {
	return z.Watcher.WriteRegister(uint16((z.ZoneNumber-1)*REG_PER_ZONE+num), value)
}

func (z *Zone) isOn() bool {
	r1 := z.readRegister(REG_ENABLED)
	return r1&0x1 != 0
}

func (z *Zone) setOn(on bool) error {
	var r1 uint16
	if on {
		r1 = 0x3
	} else {
		r1 = 0x2
	}
	return z.writeRegister(REG_ENABLED, r1)
}

func (z *Zone) isPresent() bool {
	r1 := z.readRegister(REG_ENABLED)
	return r1&0x2 != 0
}

func (z *Zone) getCurrentTemperature() float32 {
	r4 := z.readRegister(REG_CURRENT_TEMP)
	return reg2temp(r4)
}

func (z *Zone) getAverageCurrentTemperature() float32 {
	return float32(math.Round(z.temp.Avg()*10) / 10)
}

func (z *Zone) sampleTemperature() {
	sample := z.getCurrentTemperature()
	z.temp.Add(float64(sample))
	if z.OnCurrentTempChange != nil {
		t := z.getAverageCurrentTemperature()
		if t != z.lastTemp {
			z.lastTemp = t
			z.OnCurrentTempChange(t)
		}
	}
}

func (z *Zone) getTargetTemperature() float32 {
	r3 := z.readRegister(REG_TARGET_TEMP)
	return reg2temp(r3)
}

func (z *Zone) setTargetTemperature(targetTemp float32) error {
	return z.writeRegister(REG_TARGET_TEMP, temp2reg(targetTemp))
}

func (z *Zone) getFanMode() FanMode {
	r2 := z.readRegister(REG_MODE)
	return (FanMode)(r2&0x00F0) >> 4
}

func (z *Zone) setFanMode(fanMode FanMode) error {
	r2 := z.readRegister(REG_MODE) & 0x000F
	fm := (uint16(fanMode) & 0x000F) << 4
	return z.writeRegister(REG_MODE, r2|fm)
}

func (z *Zone) getKnMode() KnMode {
	r2 := z.readRegister(REG_MODE)
	return (KnMode)(r2 & 0x000F)
}

func reg2temp(r uint16) float32 {
	return float32(0x00FF&r) / 2.0
}

func temp2reg(t float32) uint16 {
	return uint16(t * 2)
}
