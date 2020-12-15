package kn

import (
	"encoding/binary"
)

type Watcher interface {
	ReadRegister(address uint16) (value []byte)
	WriteRegister(address uint16, value uint16) error
	RegisterCallback(address uint16, callback func(address uint16))
}

type ZoneConfig struct {
	ZoneNumber int
	Watcher    Watcher
}

type Zone struct {
	ZoneConfig
	OnEnabledChange     func()
	OnCurrentTempChange func(newTemp float32)
	OnTargetTempChange  func(newTemp float32)
	OnFanModeChange     func(newMode FanMode)
	OnKnModeChange      func(newMode KnMode)
}

func NewZone(config *ZoneConfig) *Zone {
	z := &Zone{
		ZoneConfig: *config,
	}
	z.RegisterCallback(REG_ENABLED, func() {
		if z.OnEnabledChange != nil {
			z.OnEnabledChange()
		}
	})
	z.RegisterCallback(REG_CURRENT_TEMP, func() {
		if z.OnCurrentTempChange == nil {
			return
		}

		temp := z.GetCurrentTemperature()
		z.OnCurrentTempChange(temp)
	})
	z.RegisterCallback(REG_TARGET_TEMP, func() {
		if z.OnTargetTempChange == nil {
			return
		}

		temp := z.GetTargetTemperature()
		z.OnTargetTempChange(temp)
	})
	z.RegisterCallback(REG_MODE, func() {
		fanMode := z.GetFanMode()
		hvacMode := z.GetKnMode()
		if z.OnFanModeChange != nil {
			z.OnFanModeChange(fanMode)
		}
		if z.OnKnModeChange != nil {
			z.OnKnModeChange(hvacMode)
		}
	})
	return z
}

func (z *Zone) RegisterCallback(num int, f func()) {
	z.Watcher.RegisterCallback(uint16(z.ZoneNumber*REG_PER_ZONE+num), func(address uint16) {
		f()
	})
}

func (z *Zone) ReadRegister(num int) uint16 {
	b := z.Watcher.ReadRegister(uint16(z.ZoneNumber*REG_PER_ZONE + num))
	return binary.BigEndian.Uint16(b)
}

func (z *Zone) WriteRegister(num int, value uint16) error {
	return z.Watcher.WriteRegister(uint16(z.ZoneNumber*REG_PER_ZONE+num), value)
}

func (z *Zone) IsOn() bool {
	r1 := z.ReadRegister(REG_ENABLED)
	return r1&0x1 != 0
}

func (z *Zone) SetOn(on bool) error {
	var r1 uint16
	if on {
		r1 = 0x3
	} else {
		r1 = 0x2
	}
	return z.WriteRegister(REG_ENABLED, r1)
}

func (z *Zone) IsPresent() bool {
	r1 := z.ReadRegister(REG_ENABLED)
	return r1&0x2 != 0
}

func (z *Zone) GetCurrentTemperature() float32 {
	r4 := z.ReadRegister(REG_CURRENT_TEMP)
	return reg2temp(r4)
}

func (z *Zone) GetTargetTemperature() float32 {
	r3 := z.ReadRegister(REG_TARGET_TEMP)
	return reg2temp(r3)
}

func (z *Zone) SetTargetTemperature(targetTemp float32) error {
	return z.WriteRegister(REG_TARGET_TEMP, temp2reg(targetTemp))
}

func (z *Zone) GetFanMode() FanMode {
	r2 := z.ReadRegister(REG_MODE)
	return (FanMode)(r2&0x00F0) >> 4
}

func (z *Zone) SetFanMode(fanMode FanMode) error {
	r2 := z.ReadRegister(REG_MODE) & 0x000F
	fm := (uint16(fanMode) & 0x000F) << 4
	return z.WriteRegister(REG_MODE, r2|fm)
}

func (z *Zone) GetKnMode() KnMode {
	r2 := z.ReadRegister(REG_MODE)
	return (KnMode)(r2 & 0x000F)
}

func reg2temp(r uint16) float32 {
	return float32(0x00FF&r) / 2.0
}

func temp2reg(t float32) uint16 {
	return uint16(t * 2)
}
