package kn

import "errors"

type SysConfig struct {
	Watcher Watcher
}

type Sys struct {
	SysConfig
	OnACAirflowChange       func(ac ACMachine)
	OnACTargetTempChange    func(ac ACMachine)
	OnACTargetFanModeChange func(ac ACMachine)
	OnEfficiencyChange      func()
	OnSystemEnabledChange   func()
	OnHvacModeChange        func()
}

var ErrUnknownSerialConfig = errors.New("Uknown serial configuration")

func NewSys(config *SysConfig) *Sys {
	s := &Sys{
		SysConfig: *config,
	}
	for n := byte(0); n < ACMachines; n++ {
		func(ac ACMachine) {
			s.Watcher.RegisterCallback(uint16(REG_AIRFLOW+ac-1), func(address uint16) {
				if s.OnACAirflowChange != nil {
					s.OnACAirflowChange(ac)
				}
			})
			s.Watcher.RegisterCallback(uint16(REG_AC_TARGET_TEMP+ac-1), func(address uint16) {
				if s.OnACTargetTempChange != nil {
					s.OnACTargetTempChange(ac)
				}
			})
			s.Watcher.RegisterCallback(uint16(REG_AC_TARGET_FAN_MODE+ac-1), func(address uint16) {
				if s.OnACTargetFanModeChange != nil {
					s.OnACTargetFanModeChange(ac)
				}
			})
		}(ACMachine(n + 1))
	}

	s.Watcher.RegisterCallback(REG_EFFICIENCY, func(address uint16) {
		if s.OnEfficiencyChange != nil {
			s.OnEfficiencyChange()
		}
	})

	s.Watcher.RegisterCallback(REG_SYSTEM_ENABLED, func(address uint16) {
		if s.OnSystemEnabledChange != nil {
			s.OnSystemEnabledChange()
		}
	})

	s.Watcher.RegisterCallback(REG_SYS_HVAC_MODE, func(address uint16) {
		if s.OnHvacModeChange != nil {
			s.OnHvacModeChange()
		}
	})

	return s
}

func (s *Sys) ReadRegister(n int) (int, error) {
	r, err := s.Watcher.ReadRegister(uint16(n))
	if err != nil || len(r) != 2 {
		return 0, nil
	}
	return int(r[1]), nil
}

func (s *Sys) GetAirflow(ac ACMachine) (int, error) {
	r, err := s.ReadRegister(REG_AIRFLOW + int(ac) - 1)
	if err != nil {
		return 0, err
	}
	return r, nil
}

func (s *Sys) GetMachineTargetTemp(ac ACMachine) (float32, error) {
	r, err := s.ReadRegister(REG_AC_TARGET_TEMP + int(ac) - 1)
	if err != nil {
		return 0, err
	}
	return reg2temp(uint16(r)), nil
}

func (s *Sys) GetTargetFanMode(ac ACMachine) (FanMode, error) {
	r, err := s.ReadRegister(REG_AC_TARGET_FAN_MODE + int(ac) - 1)
	if err != nil {
		return 0, err
	}
	return FanMode(r), nil
}

func (s *Sys) GetBaudRate() (int, error) {
	r, err := s.ReadRegister(REG_SERIAL_CONFIG)
	if err != nil {
		return 0, err
	}
	switch r {
	case 2, 6:
		return 9600, nil
	case 3, 7:
		return 19200, nil
	}
	return 0, ErrUnknownSerialConfig
}

func (s *Sys) GetParity() (string, error) {
	r, err := s.ReadRegister(REG_SERIAL_CONFIG)
	if err != nil {
		return "", err
	}
	switch r {
	case 2, 3:
		return "even", nil
	case 6, 7:
		return "none", nil
	}
	return "", ErrUnknownSerialConfig
}

func (s *Sys) GetSlaveID() (int, error) {
	r, err := s.ReadRegister(REG_SLAVE_ID)
	if err != nil {
		return 0, err
	}
	return r, nil
}

func (s *Sys) GetEfficiency() (int, error) {
	r, err := s.ReadRegister(REG_EFFICIENCY)
	if err != nil {
		return 0, err
	}
	return r, nil
}

func (s *Sys) GetSystemEnabled() (bool, error) {
	r, err := s.ReadRegister(REG_SYSTEM_ENABLED)
	if err != nil {
		return false, err
	}
	return r != 0, nil
}

func (s *Sys) GetSystemHVACMode() (HvacMode, error) {
	r, err := s.ReadRegister(REG_SYS_HVAC_MODE)
	if err != nil {
		return 0, err
	}
	return HvacMode(r), nil
}
