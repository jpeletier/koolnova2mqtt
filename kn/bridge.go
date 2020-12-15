package kn

import (
	"encoding/json"
	"fmt"
	"koolnova2mqtt/watcher"
	"log"
	"strconv"
)

type Publish func(topic string, qos byte, retained bool, payload string)
type Subscribe func(topic string, callback func(message string)) error

type Config struct {
	ModuleName    string
	SlaveID       byte
	Publish       Publish
	Subscribe     Subscribe
	TopicPrefix   string
	HassPrefix    string
	ReadRegister  watcher.ReadRegister
	WriteRegister watcher.WriteRegister
}

type Bridge struct {
	Config
	zw      *watcher.Watcher
	sysw    *watcher.Watcher
	refresh func()
}

func getActiveZones(w Watcher) ([]*Zone, error) {
	var zones []*Zone

	for n := 0; n < NUM_ZONES; n++ {
		zone := NewZone(&ZoneConfig{
			ZoneNumber: n,
			Watcher:    w,
		})
		isPresent := zone.IsPresent()
		if isPresent {
			zones = append(zones, zone)
			temp := zone.GetCurrentTemperature()
			fmt.Printf("Zone %d is present. Temperature %g ºC\n", zone.ZoneNumber, temp)
		}
	}
	return zones, nil
}

func NewBridge(config *Config) *Bridge {

	zw := watcher.New(&watcher.Config{
		Address:      FIRST_ZONE_REGISTER,
		Quantity:     TOTAL_ZONE_REGISTERS,
		RegisterSize: 2,
		SlaveID:      config.SlaveID,
		Read:         config.ReadRegister,
		Write:        config.WriteRegister,
	})

	sysw := watcher.New(&watcher.Config{
		Address:      FIRST_SYS_REGISTER,
		Quantity:     TOTAL_SYS_REGISTERS,
		RegisterSize: 2,
		SlaveID:      config.SlaveID,
		Read:         config.ReadRegister,
		Write:        config.WriteRegister,
	})

	b := &Bridge{
		Config: *config,
		zw:     zw,
		sysw:   sysw,
	}

	return b
}

func (b *Bridge) Start() error {
	sys := NewSys(&SysConfig{
		Watcher: b.sysw,
	})

	err := b.zw.Poll()
	if err != nil {
		fmt.Println(err)
		return err
	}

	err = b.sysw.Poll()
	if err != nil {
		fmt.Println(err)
		return err
	}

	getHVACMode := func() string {
		if !sys.GetSystemEnabled() {
			return HVAC_MODE_OFF
		}
		switch sys.GetSystemKNMode() {
		case MODE_AIR_COOLING, MODE_UNDERFLOOR_AIR_COOLING:
			return HVAC_MODE_COOL
		case MODE_AIR_HEATING, MODE_UNDERFLOOR_HEATING, MODE_UNDERFLOOR_AIR_HEATING:
			return HVAC_MODE_HEAT
		}
		return "unknown"
	}

	getHoldMode := func() string {
		switch sys.GetSystemKNMode() {
		case MODE_AIR_COOLING, MODE_AIR_HEATING:
			return HOLD_MODE_FAN_ONLY
		case MODE_UNDERFLOOR_HEATING:
			return HOLD_MODE_UNDERFLOOR_ONLY
		case MODE_UNDERFLOOR_AIR_COOLING, MODE_UNDERFLOOR_AIR_HEATING:
			return HOLD_MODE_UNDERFLOOR_AND_FAN
		}
		return "unknown"
	}

	zones, err := getActiveZones(b.zw)

	publishHvacMode := func() {
		for _, zone := range zones {
			if zone.IsOn() {
				hvacModeTopic := b.getZoneTopic(zone.ZoneNumber, "hvacMode")
				mode := getHVACMode()
				b.Publish(hvacModeTopic, 0, true, mode)
			}
		}
	}

	var hvacModes []string
	for k, _ := range KnModes.GetForwardMap() {
		hvacModes = append(hvacModes, k.(string))
	}

	holdModeTopic := b.getSysTopic("holdMode")
	holdModeSetTopic := holdModeTopic + "/set"

	for _, zone := range zones {
		zone := zone
		currentTempTopic := b.getZoneTopic(zone.ZoneNumber, "currentTemp")
		targetTempTopic := b.getZoneTopic(zone.ZoneNumber, "targetTemp")
		targetTempSetTopic := targetTempTopic + "/set"
		fanModeTopic := b.getZoneTopic(zone.ZoneNumber, "fanMode")
		fanModeSetTopic := fanModeTopic + "/set"
		hvacModeTopic := b.getZoneTopic(zone.ZoneNumber, "hvacMode")
		hvacModeTopicSet := hvacModeTopic + "/set"

		zone.OnCurrentTempChange = func(currentTemp float32) {
			b.Publish(currentTempTopic, 0, true, fmt.Sprintf("%g", currentTemp))
		}
		zone.OnTargetTempChange = func(targetTemp float32) {
			b.Publish(targetTempTopic, 0, true, fmt.Sprintf("%g", targetTemp))
		}
		zone.OnFanModeChange = func(fanMode FanMode) {
			b.Publish(fanModeTopic, 0, true, FanMode2Str(fanMode))
		}
		zone.OnKnModeChange = func(knMode KnMode) {

		}

		err = b.Subscribe(targetTempSetTopic, func(message string) {
			targetTemp, err := strconv.ParseFloat(message, 32)
			if err != nil {
				log.Printf("Error parsing targetTemperature in topic %s: %s", targetTempSetTopic, err)
				return
			}
			err = zone.SetTargetTemperature(float32(targetTemp))
			if err != nil {
				log.Printf("Cannot set target temperature to %g in zone %d", targetTemp, zone.ZoneNumber)
			}
		})

		name := fmt.Sprintf("%s_zone%d", b.ModuleName, zone.ZoneNumber)
		config := map[string]interface{}{
			"name":                      name,
			"current_temperature_topic": currentTempTopic,
			"precision":                 0.5,
			"temperature_state_topic":   targetTempTopic,
			"temperature_command_topic": targetTempSetTopic,
			"temperature_unit":          "C",
			"temp_step":                 0.5,
			"unique_id":                 name,
			"min_temp":                  15,
			"max_temp":                  35,
			"modes":                     []string{HVAC_MODE_COOL, HVAC_MODE_HEAT, HVAC_MODE_OFF},
			"mode_state_topic":          hvacModeTopic,
			"mode_command_topic":        hvacModeTopicSet,
			"fan_modes":                 []string{"auto", "low", "medium", "high"},
			"fan_mode_state_topic":      fanModeTopic,
			"fan_mode_command_topic":    fanModeSetTopic,
			"hold_modes":                []string{HOLD_MODE_UNDERFLOOR_ONLY, HOLD_MODE_FAN_ONLY, HOLD_MODE_UNDERFLOOR_AND_FAN},
			"hold_state_topic":          holdModeTopic,
			"hold_command_topic":        holdModeSetTopic,
		}

		configJSON, _ := json.Marshal(config)
		// <discovery_prefix>/<component>/[<node_id>/]<object_id>/config
		b.Publish(fmt.Sprintf("%s/climate/%s/zone%d/config", b.HassPrefix, b.ModuleName, zone.ZoneNumber), 0, true, string(configJSON))
	}

	sys.OnACAirflowChange = func(ac ACMachine) {
		airflow := sys.GetAirflow(ac)
		b.Publish(b.getACTopic(ac, "airflow"), 0, true, strconv.Itoa(airflow))
	}
	sys.OnACTargetTempChange = func(ac ACMachine) {
		targetTemp := sys.GetMachineTargetTemp(ac)
		b.Publish(b.getACTopic(ac, "targetTemp"), 0, true, fmt.Sprintf("%g", targetTemp))
	}
	sys.OnACTargetFanModeChange = func(ac ACMachine) {
		targetAirflow := sys.GetTargetFanMode(ac)
		b.Publish(b.getACTopic(ac, "fanMode"), 0, true, FanMode2Str(targetAirflow))
	}
	sys.OnEfficiencyChange = func() {
		efficiency := sys.GetEfficiency()
		b.Publish(b.getSysTopic("efficiency"), 0, true, strconv.Itoa(efficiency))
	}
	sys.OnSystemEnabledChange = func() {
		enabled := sys.GetSystemEnabled()
		b.Publish(b.getSysTopic("enabled"), 0, true, fmt.Sprintf("%t", enabled))
		publishHvacMode()
	}
	sys.OnKnModeChange = func() {
		publishHvacMode()
		b.Publish(holdModeTopic, 0, true, getHoldMode())
	}

	b.zw.TriggerCallbacks()
	b.sysw.TriggerCallbacks()

	b.Publish(b.getSysTopic("serialBaud"), 0, true, strconv.Itoa(sys.GetBaudRate()))
	b.Publish(b.getSysTopic("serialParity"), 0, true, sys.GetParity())
	b.Publish(b.getSysTopic("slaveId"), 0, true, strconv.Itoa(sys.GetSlaveID()))
	return nil
}

func (b *Bridge) Tick() {
	err := b.zw.Poll()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = b.sysw.Poll()
	if err != nil {
		fmt.Println(err)
		return
	}
}

func (b *Bridge) getZoneTopic(zoneNum int, subtopic string) string {
	return fmt.Sprintf("%s/%s/zone%d/%s", b.TopicPrefix, b.ModuleName, zoneNum, subtopic)
}

func (b *Bridge) getSysTopic(subtopic string) string {
	return fmt.Sprintf("%s/%s/sys/%s", b.TopicPrefix, b.ModuleName, subtopic)
}

func (b *Bridge) getACTopic(ac ACMachine, subtopic string) string {
	return b.getSysTopic(fmt.Sprintf("ac%d/%s", ac, subtopic))
}
