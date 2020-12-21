package kn

import "errors"

type SysConfig struct {
	Watcher Watcher
}

// SysDriver watches system registers and allows
// to read/change the module configuration
// notifies callbacks when specific system registers change
type SysDriver struct {
	SysConfig
	OnACAirflowChange       func(ac ACMachine)
	OnACTargetTempChange    func(ac ACMachine)
	OnACTargetFanModeChange func(ac ACMachine)
	OnEfficiencyChange      func()
	OnSystemEnabledChange   func()
	OnKnModeChange          func()
}

var ErrUnknownSerialConfig = errors.New("Uknown serial configuration")

func NewSys(config *SysConfig) *SysDriver {
	s := &SysDriver{
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

func (s *SysDriver) ReadRegister(n int) int {
	r := s.Watcher.ReadRegister(uint16(n))
	return int(r)
}

func (s *SysDriver) WriteRegister(n int, value uint16) error {
	return s.Watcher.WriteRegister(uint16(n), value)
}

func (s *SysDriver) GetAirflow(ac ACMachine) int {
	r := s.ReadRegister(REG_AIRFLOW + int(ac) - 1)
	return r
}

func (s *SysDriver) GetMachineTargetTemp(ac ACMachine) float32 {
	r := s.ReadRegister(REG_AC_TARGET_TEMP + int(ac) - 1)
	return reg2temp(uint16(r))
}

func (s *SysDriver) GetTargetFanMode(ac ACMachine) FanMode {
	r := s.ReadRegister(REG_AC_TARGET_FAN_MODE + int(ac) - 1)
	return FanMode(r)
}

func (s *SysDriver) GetBaudRate() int {
	r := s.ReadRegister(REG_SERIAL_CONFIG)
	switch r {
	case 2, 6:
		return 9600
	case 3, 7:
		return 19200
	}
	return 0
}

func (s *SysDriver) GetParity() string {
	r := s.ReadRegister(REG_SERIAL_CONFIG)
	switch r {
	case 2, 3:
		return "even"
	case 6, 7:
		return "none"
	}
	return "unknown"
}

func (s *SysDriver) GetSlaveID() int {
	r := s.ReadRegister(REG_SLAVE_ID)
	return r
}

func (s *SysDriver) GetEfficiency() int {
	r := s.ReadRegister(REG_EFFICIENCY)
	return r
}

func (s *SysDriver) GetSystemEnabled() bool {
	r := s.ReadRegister(REG_SYSTEM_ENABLED)
	return r != 0
}

func (s *SysDriver) GetSystemKNMode() KnMode {
	r := s.ReadRegister(REG_SYS_KN_MODE)
	return KnMode(r)
}

func (s *SysDriver) SetSystemKNMode(knMode KnMode) error {
	return s.WriteRegister(REG_SYS_KN_MODE, uint16(knMode))
}

// HVACMode returns the HA HVAC mode based on the
// module state
func (s *SysDriver) HVACMode() string {
	if !s.GetSystemEnabled() {
		return HVAC_MODE_OFF
	}
	switch s.GetSystemKNMode() {
	case MODE_AIR_COOLING, MODE_UNDERFLOOR_AIR_COOLING:
		return HVAC_MODE_COOL
	case MODE_AIR_HEATING, MODE_UNDERFLOOR_HEATING, MODE_UNDERFLOOR_AIR_HEATING:
		return HVAC_MODE_HEAT
	}
	return "unknown"
}

// HoldMode returns the HA Hold Mode based on the
// module state
func (s *SysDriver) HoldMode() string {
	switch s.GetSystemKNMode() {
	case MODE_AIR_COOLING, MODE_AIR_HEATING:
		return HOLD_MODE_FAN_ONLY
	case MODE_UNDERFLOOR_HEATING:
		return HOLD_MODE_UNDERFLOOR_ONLY
	case MODE_UNDERFLOOR_AIR_COOLING, MODE_UNDERFLOOR_AIR_HEATING:
		return HOLD_MODE_UNDERFLOOR_AND_FAN
	}
	return "unknown"
}
