package main

import (
	"crypto/tls"
	"flag"
	"fmt"
	"koolnova2mqtt/kn"
	"koolnova2mqtt/watcher"
	"log"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/goburrow/modbus"
)

const NUM_ZONES = 16

func buildReader(handler *modbus.RTUClientHandler, client modbus.Client) watcher.ReadRegister {
	return func(slaveID byte, address uint16, quantity uint16) (results []byte, err error) {
		handler.SlaveId = slaveID
		results, err = client.ReadHoldingRegisters(address-1, quantity)
		return results, err
	}
}

func getActiveZones(w kn.Watcher) ([]*kn.Zone, error) {
	var zones []*kn.Zone

	for n := 0; n < NUM_ZONES; n++ {
		zone := kn.NewZone(&kn.ZoneConfig{
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

func main() {

	c := make(chan os.Signal, 1)
	signal.Notify(c, os.Interrupt, syscall.SIGTERM)

	hostname, _ := os.Hostname()

	server := flag.String("server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	//topic := flag.String("topic", "#", "Topic to subscribe to")
	//qos := flag.Int("qos", 0, "The QoS to subscribe to messages at")
	clientid := flag.String("clientid", hostname+strconv.Itoa(time.Now().Second()), "A clientid for the connection")
	username := flag.String("username", "", "A username to authenticate to the MQTT server")
	password := flag.String("password", "", "Password to match username")
	prefix := flag.String("prefix", "koolnova2mqtt", "MQTT topic root where to publish/read topics")
	flag.Parse()

	// Modbus RTU/ASCII
	handler := modbus.NewRTUClientHandler("/dev/remserial1")
	handler.BaudRate = 9600
	handler.DataBits = 8
	handler.Parity = "E"
	handler.StopBits = 1
	handler.SlaveId = 49
	handler.Timeout = 5 * time.Second

	err := handler.Connect()
	if err != nil {
		fmt.Println(err)
	}
	defer handler.Close()

	client := modbus.NewClient(handler)

	nodeName := "topFloors"

	getZoneTopic := func(zoneNum int, subtopic string) string {
		return fmt.Sprintf("%s/%s/zone%d/%s", *prefix, nodeName, zoneNum, subtopic)
	}

	getSysTopic := func(subtopic string) string {
		return fmt.Sprintf("%s/%s/sys/%s", *prefix, nodeName, subtopic)
	}

	getACTopic := func(ac kn.ACMachine, subtopic string) string {
		return getSysTopic(fmt.Sprintf("ac%d/%s", ac, subtopic))
	}

	registerReader := buildReader(handler, client)

	zw := watcher.New(&watcher.Config{
		Address:      1,
		Quantity:     64,
		RegisterSize: 2,
		SlaveID:      49,
		Read:         registerReader,
	})

	sysw := watcher.New(&watcher.Config{
		Address:      kn.REG_AIRFLOW,
		Quantity:     18,
		RegisterSize: 2,
		SlaveID:      49,
		Read:         registerReader,
	})

	err = zw.Poll()
	if err != nil {
		fmt.Println(err)
		return
	}

	err = sysw.Poll()
	if err != nil {
		fmt.Println(err)
		return
	}

	zones, err := getActiveZones(zw)
	sys := kn.NewSys(&kn.SysConfig{
		Watcher: sysw,
	})

	connOpts := MQTT.NewClientOptions().AddBroker(*server).SetClientID(*clientid).SetCleanSession(true)
	if *username != "" {
		connOpts.SetUsername(*username)
		if *password != "" {
			connOpts.SetPassword(*password)
		}
	}
	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)

	connOpts.OnConnect = func(c MQTT.Client) {
		/* 		if token := c.Subscribe(*topic, byte(*qos), onMessageReceived); token.Wait() && token.Error() != nil {
			panic(token.Error())
		} */
		zw.TriggerCallbacks()
		sysw.TriggerCallbacks()
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
		c.Publish(getSysTopic("serialBaud"), 0, false, strconv.Itoa(bauds))
		c.Publish(getSysTopic("serialParity"), 0, false, parity)
		c.Publish(getSysTopic("slaveId"), 0, false, slaveID)

	}
	mqttClient := MQTT.NewClient(connOpts)

	for _, zone := range zones {
		zone := zone
		zone.OnCurrentTempChange = func(currentTemp float32) {
			mqttClient.Publish(getZoneTopic(zone.ZoneNumber, "currentTemp"), 0, false, fmt.Sprintf("%g", currentTemp))
		}
		zone.OnTargetTempChange = func(targetTemp float32) {
			mqttClient.Publish(getZoneTopic(zone.ZoneNumber, "targetTemp"), 0, false, fmt.Sprintf("%g", targetTemp))
		}
		zone.OnFanModeChange = func(fanMode kn.FanMode) {
			mqttClient.Publish(getZoneTopic(zone.ZoneNumber, "fanMode"), 0, false, kn.FanMode2Str(fanMode))
		}
		zone.OnHvacModeChange = func(hvacMode kn.HvacMode) {
			mqttClient.Publish(getZoneTopic(zone.ZoneNumber, "hvacMode"), 0, false, kn.HvacMode2Str(hvacMode))
		}
	}

	sys.OnACAirflowChange = func(ac kn.ACMachine) {
		airflow, err := sys.GetAirflow(ac)
		if err != nil {
			log.Printf("Error reading airflow of AC %d: %s", ac, err)
			return
		}
		mqttClient.Publish(getACTopic(ac, "airflow"), 0, false, strconv.Itoa(airflow))
	}
	sys.OnACTargetTempChange = func(ac kn.ACMachine) {
		targetTemp, err := sys.GetMachineTargetTemp(ac)
		if err != nil {
			log.Printf("Error reading target temp of AC %d: %s", ac, err)
			return
		}
		mqttClient.Publish(getACTopic(ac, "targetTemp"), 0, false, fmt.Sprintf("%g", targetTemp))
	}
	sys.OnACTargetFanModeChange = func(ac kn.ACMachine) {
		targetAirflow, err := sys.GetTargetFanMode(ac)
		if err != nil {
			log.Printf("Error reading target airflow of AC %d: %s", ac, err)
			return
		}
		mqttClient.Publish(getACTopic(ac, "fanMode"), 0, false, kn.FanMode2Str(targetAirflow))
	}
	sys.OnEfficiencyChange = func() {
		efficiency, err := sys.GetEfficiency()
		if err != nil {
			log.Printf("Error reading efficiency value: %s", err)
			return
		}
		mqttClient.Publish(getSysTopic("efficiency"), 0, false, strconv.Itoa(efficiency))
	}
	sys.OnSystemEnabledChange = func() {
		enabled, err := sys.GetSystemEnabled()
		if err != nil {
			log.Printf("Error reading enabled value: %s", err)
			return
		}
		mqttClient.Publish(getSysTopic("enabled"), 0, false, fmt.Sprintf("%t", enabled))
	}
	sys.OnHvacModeChange = func() {
		mode, err := sys.GetSystemHVACMode()
		if err != nil {
			log.Printf("Error reading hvac mode: %s", err)
			return
		}
		mqttClient.Publish(getSysTopic("hvacMode"), 0, false, kn.HvacMode2Str(mode))
	}

	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to %s\n", *server)
	}

	ticker := time.NewTicker(time.Second)

	go func() {
		for range ticker.C {
			err := zw.Poll()
			if err != nil {
				log.Printf("Error polling zones over modbus: %s\n", err)
			}

			err = sysw.Poll()
			if err != nil {
				log.Printf("Error polling system config over modbus: %s\n", err)
			}
		}
	}()

	<-c

}
