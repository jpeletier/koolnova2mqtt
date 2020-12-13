package kn

import (
	"encoding/binary"
	"log"
)

type Watcher interface {
	ReadRegister(address uint16) (value []byte, err error)
	RegisterCallback(address uint16, callback func(address uint16))
}

type ZoneConfig struct {
	ZoneNumber int
	Watcher    Watcher
}

type Zone struct {
	ZoneConfig
	OnCurrentTempChange func(newTemp float32)
	OnTargetTempChange  func(newTemp float32)
	OnFanModeChange     func(newMode FanMode)
	OnHvacModeChange    func(newMode HvacMode)
}

func NewZone(config *ZoneConfig) *Zone {
	z := &Zone{
		ZoneConfig: *config,
	}
	z.RegisterCallback(REG_CURRENT_TEMP, func() {
		if z.OnCurrentTempChange == nil {
			return
		}

		temp, err := z.GetCurrentTemperature()
		if err != nil {
			log.Printf("Cannot read current temperature: %s\n", err)
			return
		}
		z.OnCurrentTempChange(temp)
	})
	z.RegisterCallback(REG_TARGET_TEMP, func() {
		if z.OnTargetTempChange == nil {
			return
		}

		temp, err := z.GetTargetTemperature()
		if err != nil {
			log.Printf("Cannot read target temperature: %s\n", err)
			return
		}
		z.OnTargetTempChange(temp)
	})
	z.RegisterCallback(REG_MODE, func() {
		fanMode, err := z.GetFanMode()
		if err != nil {
			log.Printf("Cannot read fan mode: %s\n", err)
			return
		}
		hvacMode, err := z.GetHvacMode()
		if err != nil {
			log.Printf("Cannot hvac mode: %s\n", err)
			return
		}
		if z.OnFanModeChange != nil {
			z.OnFanModeChange(fanMode)
		}
		if z.OnHvacModeChange != nil {
			z.OnHvacModeChange(hvacMode)
		}
	})
	return z
}

func (z *Zone) RegisterCallback(num int, f func()) {
	z.Watcher.RegisterCallback(uint16(z.ZoneNumber*REG_PER_ZONE+num), func(address uint16) {
		f()
	})
}

func (z *Zone) ReadRegister(num int) (uint16, error) {

	b, err := z.Watcher.ReadRegister(uint16(z.ZoneNumber*REG_PER_ZONE + num))
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

func (z *Zone) IsOn() (bool, error) {
	r1, err := z.ReadRegister(REG_ENABLED)
	if err != nil {
		return false, err
	}

	return r1&uint16(0x1) != 0, nil
}

func (z *Zone) IsPresent() (bool, error) {
	r1, err := z.ReadRegister(REG_ENABLED)
	if err != nil {
		return false, err
	}

	return r1&uint16(0x2) != 0, nil
}

func (z *Zone) GetCurrentTemperature() (float32, error) {
	r4, err := z.ReadRegister(REG_CURRENT_TEMP)
	if err != nil {
		return 0.0, err
	}

	return float32(r4) / 2.0, nil
}

func (z *Zone) GetTargetTemperature() (float32, error) {
	r3, err := z.ReadRegister(REG_TARGET_TEMP)
	if err != nil {
		return 0.0, err
	}
	return float32(r3) / 2.0, nil
}

func (z *Zone) GetFanMode() (FanMode, error) {
	r2, err := z.ReadRegister(REG_MODE)
	if err != nil {
		return 0, err
	}
	return (FanMode)(r2 & 0x00F0), nil
}

func (z *Zone) GetHvacMode() (HvacMode, error) {
	r2, err := z.ReadRegister(REG_MODE)
	if err != nil {
		return 0, err
	}
	return (HvacMode)(r2 & 0x000F), nil
}
