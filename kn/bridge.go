package kn

import (
	"fmt"
	"koolnova2mqtt/watcher"
	"log"
	"strconv"
)

type Publish func(topic string, qos byte, retained bool, payload string)

type Config struct {
	ModuleName   string
	SlaveID      byte
	Publish      Publish
	TopicPrefix  string
	ReadRegister watcher.ReadRegister
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
		isPresent, err := zone.IsPresent()
		if err != nil {
			return nil, err
		}
		if isPresent {
			zones = append(zones, zone)
			temp, _ := zone.GetCurrentTemperature()
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
	})

	sysw := watcher.New(&watcher.Config{
		Address:      FIRST_SYS_REGISTER,
		Quantity:     TOTAL_SYS_REGISTERS,
		RegisterSize: 2,
		SlaveID:      config.SlaveID,
		Read:         config.ReadRegister,
	})

	b := &Bridge{
		Config: *config,
		zw:     zw,
		sysw:   sysw,
	}

	return b
}

func (b *Bridge) Start() {
	sys := NewSys(&SysConfig{
		Watcher: b.sysw,
	})

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

	zones, err := getActiveZones(b.zw)

	for _, zone := range zones {
		zone := zone
		zone.OnCurrentTempChange = func(currentTemp float32) {
			b.Publish(b.getZoneTopic(zone.ZoneNumber, "currentTemp"), 0, false, fmt.Sprintf("%g", currentTemp))
		}
		zone.OnTargetTempChange = func(targetTemp float32) {
			b.Publish(b.getZoneTopic(zone.ZoneNumber, "targetTemp"), 0, false, fmt.Sprintf("%g", targetTemp))
		}
		zone.OnFanModeChange = func(fanMode FanMode) {
			b.Publish(b.getZoneTopic(zone.ZoneNumber, "fanMode"), 0, false, FanMode2Str(fanMode))
		}
		zone.OnHvacModeChange = func(hvacMode HvacMode) {
			b.Publish(b.getZoneTopic(zone.ZoneNumber, "hvacMode"), 0, false, HvacMode2Str(hvacMode))
		}
	}

	sys.OnACAirflowChange = func(ac ACMachine) {
		airflow, err := sys.GetAirflow(ac)
		if err != nil {
			log.Printf("Error reading airflow of AC %d: %s", ac, err)
			return
		}
		b.Publish(b.getACTopic(ac, "airflow"), 0, false, strconv.Itoa(airflow))
	}
	sys.OnACTargetTempChange = func(ac ACMachine) {
		targetTemp, err := sys.GetMachineTargetTemp(ac)
		if err != nil {
			log.Printf("Error reading target temp of AC %d: %s", ac, err)
			return
		}
		b.Publish(b.getACTopic(ac, "targetTemp"), 0, false, fmt.Sprintf("%g", targetTemp))
	}
	sys.OnACTargetFanModeChange = func(ac ACMachine) {
		targetAirflow, err := sys.GetTargetFanMode(ac)
		if err != nil {
			log.Printf("Error reading target airflow of AC %d: %s", ac, err)
			return
		}
		b.Publish(b.getACTopic(ac, "fanMode"), 0, false, FanMode2Str(targetAirflow))
	}
	sys.OnEfficiencyChange = func() {
		efficiency, err := sys.GetEfficiency()
		if err != nil {
			log.Printf("Error reading efficiency value: %s", err)
			return
		}
		b.Publish(b.getSysTopic("efficiency"), 0, false, strconv.Itoa(efficiency))
	}
	sys.OnSystemEnabledChange = func() {
		enabled, err := sys.GetSystemEnabled()
		if err != nil {
			log.Printf("Error reading enabled value: %s", err)
			return
		}
		b.Publish(b.getSysTopic("enabled"), 0, false, fmt.Sprintf("%t", enabled))
	}
	sys.OnHvacModeChange = func() {
		mode, err := sys.GetSystemHVACMode()
		if err != nil {
			log.Printf("Error reading hvac mode: %s", err)
			return
		}
		b.Publish(b.getSysTopic("hvacMode"), 0, false, HvacMode2Str(mode))
	}

	b.zw.TriggerCallbacks()
	b.sysw.TriggerCallbacks()

	bauds, err := sys.GetBaudRate()
	if err != nil {
		log.Printf("Error reading configured serial baud rate: %s", err)
	}
	parity, err := sys.GetParity()
	if err != nil {
		log.Printf("Error reading configured serial parity: %s", err)
	}
	slaveID, err := sys.GetSlaveID()
	if err != nil {
		log.Printf("Error reading configured modbus slave ID: %s", err)
	}
	b.Publish(b.getSysTopic("serialBaud"), 0, false, strconv.Itoa(bauds))
	b.Publish(b.getSysTopic("serialParity"), 0, false, parity)
	b.Publish(b.getSysTopic("slaveId"), 0, false, strconv.Itoa(slaveID))
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
