package kn

import (
	"encoding/json"
	"fmt"
	"koolnova2mqtt/modbus"
	"koolnova2mqtt/watcher"
	"log"
	"strconv"
)

type MqttClient interface {
	Publish(topic string, qos byte, retained bool, payload string) error
	Subscribe(topic string, callback func(message string)) error
}

type Config struct {
	ModuleName  string
	SlaveID     byte
	Mqtt        MqttClient
	TopicPrefix string
	HassPrefix  string
	Modbus      modbus.Modbus
}

type Bridge struct {
	Config
	zw      *watcher.Watcher
	sysw    *watcher.Watcher
	refresh func()
	zones   []*Zone
}

func getActiveZones(w Watcher) ([]*Zone, error) {
	var zones []*Zone

	for n := 0; n < NUM_ZONES; n++ {
		zone := NewZone(&ZoneConfig{
			ZoneNumber: n + 1,
			Watcher:    w,
		})
		isPresent := zone.IsPresent()
		if isPresent {
			zones = append(zones, zone)
		}
	}
	return zones, nil
}

func NewBridge(config *Config) *Bridge {
	b := &Bridge{
		Config: *config,
	}
	return b
}

func (b *Bridge) Start() error {

	zw := watcher.New(&watcher.Config{
		Address:      FIRST_ZONE_REGISTER,
		Quantity:     TOTAL_ZONE_REGISTERS,
		RegisterSize: 2,
		SlaveID:      b.SlaveID,
		Modbus:       b.Modbus,
	})

	sysw := watcher.New(&watcher.Config{
		Address:      FIRST_SYS_REGISTER,
		Quantity:     TOTAL_SYS_REGISTERS,
		RegisterSize: 2,
		SlaveID:      b.SlaveID,
		Modbus:       b.Modbus,
	})
	b.zw = zw
	b.sysw = sysw
	sys := NewSys(&SysConfig{
		Watcher: b.sysw,
	})

	log.Printf("Starting bridge for %s\n", b.ModuleName)
	err := b.poll()
	if err != nil {
		return err
	}

	zones, err := getActiveZones(b.zw)
	b.zones = zones
	log.Printf("%d zones are present in %s\n", len(zones), b.ModuleName)
	b.zw.Resize(len(zones) * REG_PER_ZONE)

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

	publishHvacMode := func() {
		for _, zone := range zones {
			if zone.IsOn() {
				hvacModeTopic := b.getZoneTopic(zone.ZoneNumber, "hvacMode")
				mode := getHVACMode()
				b.Mqtt.Publish(hvacModeTopic, 0, true, mode)
			}
		}
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
		hvacModeSetTopic := hvacModeTopic + "/set"

		zone.OnEnabledChange = func() {
			hvacModeTopic := b.getZoneTopic(zone.ZoneNumber, "hvacMode")
			var mode string
			if zone.IsOn() {
				mode = getHVACMode()
			} else {
				mode = HVAC_MODE_OFF
			}
			b.Mqtt.Publish(hvacModeTopic, 0, true, mode)
		}
		zone.OnCurrentTempChange = func(currentTemp float32) {
			b.Mqtt.Publish(currentTempTopic, 0, true, fmt.Sprintf("%g", currentTemp))
		}
		zone.OnTargetTempChange = func(targetTemp float32) {
			b.Mqtt.Publish(targetTempTopic, 0, true, fmt.Sprintf("%g", targetTemp))
		}
		zone.OnFanModeChange = func(fanMode FanMode) {
			b.Mqtt.Publish(fanModeTopic, 0, true, FanMode2Str(fanMode))
		}
		zone.OnKnModeChange = func(knMode KnMode) {

		}

		err = b.Mqtt.Subscribe(targetTempSetTopic, func(message string) {
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
		if err != nil {
			return err
		}

		err = b.Mqtt.Subscribe(fanModeSetTopic, func(message string) {
			fm, err := Str2FanMode(message)
			if err != nil {
				log.Printf("Unknown fan mode %q in message to zone %d", message, zone.ZoneNumber)
			}
			err = zone.SetFanMode(fm)
			if err != nil {
				log.Printf("Cannot set fan mode to %s in zone %d", message, zone.ZoneNumber)
			}
		})
		if err != nil {
			return err
		}

		err = b.Mqtt.Subscribe(hvacModeSetTopic, func(message string) {
			if message == HVAC_MODE_OFF {
				err := zone.SetOn(false)
				if err != nil {
					log.Printf("Cannot set zone %d to off", zone.ZoneNumber)
				}
				return
			}
			knMode := sys.GetSystemKNMode()
			knMode = ApplyHvacMode(knMode, message)
			err = sys.SetSystemKNMode(knMode)
			if err != nil {
				log.Printf("Cannot set knmode mode to %x in zone %d", knMode, zone.ZoneNumber)
			}
			err := zone.SetOn(true)
			if err != nil {
				log.Printf("Cannot set zone %d to on", zone.ZoneNumber)
				return
			}
		})
		if err != nil {
			return err
		}

		err = b.Mqtt.Subscribe(holdModeSetTopic, func(message string) {
			knMode := sys.GetSystemKNMode()
			knMode = ApplyHoldMode(knMode, message)
			err := sys.SetSystemKNMode(knMode)
			if err != nil {
				log.Printf("Cannot set knmode mode to %x in zone %d", knMode, zone.ZoneNumber)
			}
		})
		if err != nil {
			return err
		}

		name := fmt.Sprintf("%s_zone%d", b.ModuleName, zone.ZoneNumber)
		config := map[string]interface{}{
			"name":                      name,
			"current_temperature_topic": currentTempTopic,
			"precision":                 0.1,
			"temperature_state_topic":   targetTempTopic,
			"temperature_command_topic": targetTempSetTopic,
			"temperature_unit":          "C",
			"temp_step":                 0.5,
			"unique_id":                 name,
			"min_temp":                  15,
			"max_temp":                  35,
			"modes":                     []string{HVAC_MODE_COOL, HVAC_MODE_HEAT, HVAC_MODE_OFF},
			"mode_state_topic":          hvacModeTopic,
			"mode_command_topic":        hvacModeSetTopic,
			"fan_modes":                 []string{"auto", "low", "medium", "high"},
			"fan_mode_state_topic":      fanModeTopic,
			"fan_mode_command_topic":    fanModeSetTopic,
			"hold_modes":                []string{HOLD_MODE_UNDERFLOOR_ONLY, HOLD_MODE_FAN_ONLY, HOLD_MODE_UNDERFLOOR_AND_FAN},
			"hold_state_topic":          holdModeTopic,
			"hold_command_topic":        holdModeSetTopic,
		}

		configJSON, _ := json.Marshal(config)
		// <discovery_prefix>/<component>/[<node_id>/]<object_id>/config
		b.Mqtt.Publish(fmt.Sprintf("%s/climate/%s/zone%d/config", b.HassPrefix, b.ModuleName, zone.ZoneNumber), 0, true, string(configJSON))

		// temperature sensor configuration:
		name = fmt.Sprintf("%s_zone%d_temp", b.ModuleName, zone.ZoneNumber)
		config = map[string]interface{}{
			"name":                name,
			"device_class":        "temperature",
			"state_topic":         currentTempTopic,
			"unit_of_measurement": "ºC",
			"unique_id":           name,
		}

		configJSON, _ = json.Marshal(config)
		b.Mqtt.Publish(fmt.Sprintf("%s/sensor/%s/zone%d_temp/config", b.HassPrefix, b.ModuleName, zone.ZoneNumber), 0, true, string(configJSON))

	}

	sys.OnACAirflowChange = func(ac ACMachine) {
		airflow := sys.GetAirflow(ac)
		b.Mqtt.Publish(b.getACTopic(ac, "airflow"), 0, true, strconv.Itoa(airflow))
	}
	sys.OnACTargetTempChange = func(ac ACMachine) {
		targetTemp := sys.GetMachineTargetTemp(ac)
		b.Mqtt.Publish(b.getACTopic(ac, "targetTemp"), 0, true, fmt.Sprintf("%g", targetTemp))
	}
	sys.OnACTargetFanModeChange = func(ac ACMachine) {
		targetAirflow := sys.GetTargetFanMode(ac)
		b.Mqtt.Publish(b.getACTopic(ac, "fanMode"), 0, true, FanMode2Str(targetAirflow))
	}
	sys.OnEfficiencyChange = func() {
		efficiency := sys.GetEfficiency()
		b.Mqtt.Publish(b.getSysTopic("efficiency"), 0, true, strconv.Itoa(efficiency))
	}
	sys.OnSystemEnabledChange = func() {
		enabled := sys.GetSystemEnabled()
		b.Mqtt.Publish(b.getSysTopic("enabled"), 0, true, fmt.Sprintf("%t", enabled))
		publishHvacMode()
	}
	sys.OnKnModeChange = func() {
		publishHvacMode()
		b.Mqtt.Publish(holdModeTopic, 0, true, getHoldMode())
	}

	b.zw.TriggerCallbacks()
	b.sysw.TriggerCallbacks()

	b.Mqtt.Publish(b.getSysTopic("serialBaud"), 0, true, strconv.Itoa(sys.GetBaudRate()))
	b.Mqtt.Publish(b.getSysTopic("serialParity"), 0, true, sys.GetParity())
	b.Mqtt.Publish(b.getSysTopic("slaveId"), 0, true, strconv.Itoa(sys.GetSlaveID()))
	return nil
}

func (b *Bridge) poll() error {
	err := b.zw.Poll()
	if err != nil {
		log.Printf("Timeout polling %s zone registers: %s\n", b.ModuleName, err)
		return err
	}

	err = b.sysw.Poll()
	if err != nil {
		log.Printf("Timeout polling %s system registers: %s\n", b.ModuleName, err)
		return err
	}
	return nil
}

func (b *Bridge) Tick() error {
	err := b.poll()
	if err != nil {
		return err
	}
	for _, z := range b.zones {
		z.SampleTemperature()
	}
	return nil
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
