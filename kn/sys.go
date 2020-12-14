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
	OnKnModeChange          func()
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

	s.Watcher.RegisterCallback(REG_SYS_KN_MODE, func(address uint16) {
		if s.OnKnModeChange != nil {
			s.OnKnModeChange()
		}
	})

	return s
}

func (s *Sys) ReadRegister(n int) int {
	r := s.Watcher.ReadRegister(uint16(n))
	return int(r[1])
}

func (s *Sys) GetAirflow(ac ACMachine) int {
	r := s.ReadRegister(REG_AIRFLOW + int(ac) - 1)
	return r
}

func (s *Sys) GetMachineTargetTemp(ac ACMachine) float32 {
	r := s.ReadRegister(REG_AC_TARGET_TEMP + int(ac) - 1)
	return reg2temp(uint16(r))
}

func (s *Sys) GetTargetFanMode(ac ACMachine) FanMode {
	r := s.ReadRegister(REG_AC_TARGET_FAN_MODE + int(ac) - 1)
	return FanMode(r)
}

func (s *Sys) GetBaudRate() int {
	r := s.ReadRegister(REG_SERIAL_CONFIG)
	switch r {
	case 2, 6:
		return 9600
	case 3, 7:
		return 19200
	}
	return 0
}

func (s *Sys) GetParity() string {
	r := s.ReadRegister(REG_SERIAL_CONFIG)
	switch r {
	case 2, 3:
		return "even"
	case 6, 7:
		return "none"
	}
	return "unknown"
}

func (s *Sys) GetSlaveID() int {
	r := s.ReadRegister(REG_SLAVE_ID)
	return r
}

func (s *Sys) GetEfficiency() int {
	r := s.ReadRegister(REG_EFFICIENCY)
	return r
}

func (s *Sys) GetSystemEnabled() bool {
	r := s.ReadRegister(REG_SYSTEM_ENABLED)
	return r != 0
}

func (s *Sys) GetSystemKNMode() KnMode {
	r := s.ReadRegister(REG_SYS_KN_MODE)
	return KnMode(r)
}
