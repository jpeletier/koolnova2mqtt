package kn

import (
	"encoding/json"
	"fmt"
	"koolnova2mqtt/watcher"
	"log"
	"strconv"
)

// MqttClient defines the expected MQTT client pub sub interface
type MqttClient interface {
	Publish(topic string, qos byte, retained bool, payload string) error
	Subscribe(topic string, callback func(message string)) error
}

// Config defines de Modbus<>MQTT bridge configuration
type Config struct {
	ModuleName  string         // name of the module the modbus interface is connected to
	SlaveID     byte           // SlaveID of the module in the bus
	TopicPrefix string         // MQTT topic prefix to publish information
	HassPrefix  string         // Home Assistant sensor discovery prefix
	Mqtt        MqttClient     // MQTT client
	Modbus      watcher.Modbus // Modbus client
}

// Bridge bridges Modbus and MQTT protocols
type Bridge struct {
	Config                  // embedded configuration
	zw     *watcher.Watcher // watcher to detect register changes in zones
	sysw   *watcher.Watcher // watcher to detect register changes in system registers
	zones  []*Zone          // List of present zones in this module
	sys    *SysDriver
}

// getActiveZones returns the list of active zones in this module
func getActiveZones(w Watcher) ([]*Zone, error) {
	var zones []*Zone

	for n := 0; n < NUM_ZONES; n++ {
		zone := newZone(&ZoneConfig{
			ZoneNumber: n + 1,
			Watcher:    w,
		})
		isPresent := zone.isPresent()
		if isPresent {
			zones = append(zones, zone)
		}
	}
	return zones, nil
}

// NewBridge returns a new Modbus<>MQTT bridge
func NewBridge(config *Config) *Bridge {
	b := &Bridge{
		Config: *config,
	}
	return b
}

// Start starts the bridge, publishes the configuration to Home Assistant topics and publishes
// the current state. Call Start() every time MQTT gets disconnected.
func (b *Bridge) Start() error {

	// Define a watcher to watch the zone registers
	zw := watcher.New(&watcher.Config{
		Address:  FIRST_ZONE_REGISTER,
		Quantity: TOTAL_ZONE_REGISTERS,
		SlaveID:  b.SlaveID,
		Modbus:   b.Modbus,
	})

	// Define a watcher to watch the system registers
	sysw := watcher.New(&watcher.Config{
		Address:  FIRST_SYS_REGISTER,
		Quantity: TOTAL_SYS_REGISTERS,
		SlaveID:  b.SlaveID,
		Modbus:   b.Modbus,
	})

	b.zw = zw
	b.sysw = sysw
	sys := NewSys(&SysConfig{
		Watcher: b.sysw,
	})
	b.sys = sys

	log.Printf("Starting bridge for %s\n", b.ModuleName)
	err := b.poll()
	if err != nil {
		return err
	}

	// Get Active zones
	zones, err := getActiveZones(b.zw)
	b.zones = zones
	log.Printf("%d zones are present in %s\n", len(zones), b.ModuleName)

	// Downsize watched range to the required number of registers
	b.zw.Resize(len(zones) * REG_PER_ZONE)

	holdModeTopic := b.getSysTopic("holdMode")
	holdModeSetTopic := holdModeTopic + "/set"

	// configure publishing when modbus registers change
	for _, zone := range zones {
		zone := zone
		currentTempTopic := b.getZoneTopic(zone.ZoneNumber, "currentTemp")
		targetTempTopic := b.getZoneTopic(zone.ZoneNumber, "targetTemp")
		targetTempSetTopic := targetTempTopic + "/set"
		fanModeTopic := b.getZoneTopic(zone.ZoneNumber, "fanMode")
		fanModeSetTopic := fanModeTopic + "/set"
		hvacModeTopic := b.getZoneTopic(zone.ZoneNumber, "hvacMode")
		hvacModeSetTopic := hvacModeTopic + "/set"

		// In HA there is three HVAC modes: "cool", "heat" and "off". Therefore,
		// publish "OFF" if we detect the REG_ENABLED change is off
		// Otherwise, publish "heat" or "cool" depending on REG_MODE
		zone.OnEnabledChange = func() {
			hvacModeTopic := b.getZoneTopic(zone.ZoneNumber, "hvacMode")
			var mode string
			if zone.isOn() {
				mode = sys.HVACMode()
			} else {
				mode = HVAC_MODE_OFF
			}
			b.Mqtt.Publish(hvacModeTopic, 0, true, mode)
		}

		// if the current temperature changes, forward value to the
		// correspondig MQTT topic
		zone.OnCurrentTempChange = func(currentTemp float32) {
			b.Mqtt.Publish(currentTempTopic, 0, true, fmt.Sprintf("%g", currentTemp))
		}

		// if the target temperature changes, publish it to MQTT
		// this is fired when the target is set over MQTT or via a thermostat
		zone.OnTargetTempChange = func(targetTemp float32) {
			b.Mqtt.Publish(targetTempTopic, 0, true, fmt.Sprintf("%g", targetTemp))
		}

		// Publish changes to the fan mode
		zone.OnFanModeChange = func(fanMode FanMode) {
			b.Mqtt.Publish(fanModeTopic, 0, true, FanMode2Str(fanMode))
		}

		// Subscribe to target temperature set topic in MQTT
		err = b.Mqtt.Subscribe(targetTempSetTopic, func(message string) {
			targetTemp, err := strconv.ParseFloat(message, 32)
			if err != nil {
				log.Printf("Error parsing targetTemperature in topic %s: %s", targetTempSetTopic, err)
				return
			}
			err = zone.setTargetTemperature(float32(targetTemp))
			if err != nil {
				log.Printf("Cannot set target temperature to %g in zone %d", targetTemp, zone.ZoneNumber)
			}
		})
		if err != nil {
			return err
		}

		// Subscribe to fan mode set topic in MQTT
		err = b.Mqtt.Subscribe(fanModeSetTopic, func(message string) {
			fm, err := Str2FanMode(message)
			if err != nil {
				log.Printf("Unknown fan mode %q in message to zone %d", message, zone.ZoneNumber)
			}
			err = zone.setFanMode(fm)
			if err != nil {
				log.Printf("Cannot set fan mode to %s in zone %d", message, zone.ZoneNumber)
			}
		})
		if err != nil {
			return err
		}

		// Subscribe to HVAC Mode set topic in MQTT
		err = b.Mqtt.Subscribe(hvacModeSetTopic, func(message string) {
			// If user sets the Home Assistant HVAC mode to off, turn off this zone
			if message == HVAC_MODE_OFF {
				err := zone.setOn(false) // turn zone off (REG_ENABLED)
				if err != nil {
					log.Printf("Cannot set zone %d to off", zone.ZoneNumber)
				}
				return
			}
			// Translate HA HVAC mode to Koolnova's
			knMode := sys.GetSystemKNMode()
			knMode = ApplyHvacMode(knMode, message)
			err = sys.SetSystemKNMode(knMode)
			if err != nil {
				log.Printf("Cannot set knmode mode to %x in zone %d", knMode, zone.ZoneNumber)
			}
			err := zone.setOn(true)
			if err != nil {
				log.Printf("Cannot set zone %d to on", zone.ZoneNumber)
				return
			}
		})
		if err != nil {
			return err
		}

		// Subscribe to changes in hold mode:
		err = b.Mqtt.Subscribe(holdModeSetTopic, func(message string) {
			// Translate HA's hold mode to Koolnova's
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

		// Define a Home Assistant thermostat
		name := fmt.Sprintf("%s_zone%d", b.ModuleName, zone.ZoneNumber)
		b.publishComponent(HA_COMPONENT_CLIMATE, fmt.Sprintf("zone%d", zone.ZoneNumber), map[string]interface{}{
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
		})

		// Define a current temperature sensor:
		name = fmt.Sprintf("%s_zone%d_temp", b.ModuleName, zone.ZoneNumber)
		b.publishComponent(HA_COMPONENT_SENSOR, fmt.Sprintf("zone%d_temp", zone.ZoneNumber), map[string]interface{}{
			"name":                name,
			"device_class":        "temperature",
			"state_topic":         currentTempTopic,
			"unit_of_measurement": "°C",
			"unique_id":           name,
		})

		// Define a target temperature sensor:
		name = fmt.Sprintf("%s_zone%d_target_temp", b.ModuleName, zone.ZoneNumber)
		b.publishComponent(HA_COMPONENT_SENSOR, fmt.Sprintf("zone%d_target_temp", zone.ZoneNumber), map[string]interface{}{
			"name":                name,
			"device_class":        "temperature",
			"state_topic":         targetTempTopic,
			"unit_of_measurement": "°C",
			"unique_id":           name,
		})

	}

	// Publish changes to system registers:
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
		b.publishHvacMode()
	}

	sys.OnKnModeChange = func() {
		b.publishHvacMode()
		b.Mqtt.Publish(holdModeTopic, 0, true, sys.HoldMode())
	}

	// Trigger a callback on all registers so the MQTT broker is updated on connect:
	b.zw.TriggerCallbacks()
	b.sysw.TriggerCallbacks()

	// Publish one-off static information
	b.Mqtt.Publish(b.getSysTopic("serialBaud"), 0, true, strconv.Itoa(sys.GetBaudRate()))
	b.Mqtt.Publish(b.getSysTopic("serialParity"), 0, true, sys.GetParity())
	b.Mqtt.Publish(b.getSysTopic("slaveId"), 0, true, strconv.Itoa(sys.GetSlaveID()))
	return nil
}

// poll polls modbus for changes
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

// Tick must be invoked periodically to refesh registers from modbus
// it also samples temperature and calculates a moving average of read temperatures
func (b *Bridge) Tick() error {
	err := b.poll()
	if err != nil {
		return err
	}
	for _, z := range b.zones {
		z.sampleTemperature()
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

func (b *Bridge) publishHvacMode() {
	for _, zone := range b.zones {
		if zone.isOn() {
			hvacModeTopic := b.getZoneTopic(zone.ZoneNumber, "hvacMode")
			mode := b.sys.HVACMode()
			b.Mqtt.Publish(hvacModeTopic, 0, true, mode)
		}
	}
}

// publishComponent publishes a Home Assistant component configuration for autodiscovery
func (b *Bridge) publishComponent(component, ObjectID string, config map[string]interface{}) {
	configJSON, _ := json.Marshal(config)
	b.Mqtt.Publish(fmt.Sprintf("%s/%s/%s/%s/config", b.HassPrefix, component, b.ModuleName, ObjectID), 0, true, string(configJSON))
}
