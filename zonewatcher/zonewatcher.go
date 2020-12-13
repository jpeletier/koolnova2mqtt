package zonewatcher

import (
	"encoding/binary"
	"log"
)

type Watcher interface {
	ReadRegister(address uint16) (value []byte, err error)
	RegisterCallback(address uint16, callback func(address uint16))
}

type Config struct {
	Zone    int
	Watcher Watcher
}

type ZoneWatcher struct {
	Config
	OnCurrentTempChange func(newTemp float32)
	OnTargetTempChange  func(newTemp float32)
	OnFanModeChange     func(newMode FanMode)
	OnHvacModeChange    func(newMode HvacMode)
}

func New(config *Config) *ZoneWatcher {
	zw := &ZoneWatcher{
		Config: *config,
	}
	zw.RegisterCallback(REG_CURRENT_TEMP, func() {
		if zw.OnCurrentTempChange == nil {
			return
		}

		temp, err := zw.GetCurrentTemperature()
		if err != nil {
			log.Printf("Cannot read current temperature: %s\n", err)
			return
		}
		zw.OnCurrentTempChange(temp)
	})
	zw.RegisterCallback(REG_TARGET_TEMP, func() {
		if zw.OnTargetTempChange == nil {
			return
		}

		temp, err := zw.GetTargetTemperature()
		if err != nil {
			log.Printf("Cannot read target temperature: %s\n", err)
			return
		}
		zw.OnTargetTempChange(temp)
	})
	zw.RegisterCallback(REG_MODE, func() {
		fanMode, err := zw.GetFanMode()
		if err != nil {
			log.Printf("Cannot read fan mode: %s\n", err)
			return
		}
		hvacMode, err := zw.GetHvacMode()
		if err != nil {
			log.Printf("Cannot hvac mode: %s\n", err)
			return
		}
		if zw.OnFanModeChange != nil {
			zw.OnFanModeChange(fanMode)
		}
		if zw.OnHvacModeChange != nil {
			zw.OnHvacModeChange(hvacMode)
		}
	})
	return zw
}

func (zw *ZoneWatcher) RegisterCallback(num int, f func()) {
	zw.Watcher.RegisterCallback(uint16(zw.Zone*REG_PER_ZONE+num), func(address uint16) {
		f()
	})
}

func (zw *ZoneWatcher) ReadRegister(num int) (uint16, error) {

	b, err := zw.Watcher.ReadRegister(uint16(zw.Zone*REG_PER_ZONE + num))
	if err != nil {
		return 0, err
	}
	return binary.BigEndian.Uint16(b), nil
}

func (zw *ZoneWatcher) IsOn() (bool, error) {
	r1, err := zw.ReadRegister(REG_ENABLED)
	if err != nil {
		return false, err
	}

	return r1&uint16(0x1) != 0, nil
}

func (zw *ZoneWatcher) IsPresent() (bool, error) {
	r1, err := zw.ReadRegister(REG_ENABLED)
	if err != nil {
		return false, err
	}

	return r1&uint16(0x2) != 0, nil
}

func (zw *ZoneWatcher) GetCurrentTemperature() (float32, error) {
	r4, err := zw.ReadRegister(REG_CURRENT_TEMP)
	if err != nil {
		return 0.0, err
	}

	return float32(r4) / 2.0, nil
}

func (zw *ZoneWatcher) GetTargetTemperature() (float32, error) {
	r3, err := zw.ReadRegister(REG_TARGET_TEMP)
	if err != nil {
		return 0.0, err
	}
	return float32(r3) / 2.0, nil
}

func (zw *ZoneWatcher) GetFanMode() (FanMode, error) {
	r2, err := zw.ReadRegister(REG_MODE)
	if err != nil {
		return 0, err
	}
	return (FanMode)(r2 & 0x00F0), nil
}

func (zw *ZoneWatcher) GetHvacMode() (HvacMode, error) {
	r2, err := zw.ReadRegister(REG_MODE)
	if err != nil {
		return 0, err
	}
	return (HvacMode)(r2 & 0x000F), nil
}
