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

	getTopic := func(zoneNum int, subtopic string) string {
		return fmt.Sprintf("%s/%s/zone%d/%s", *prefix, nodeName, zoneNum, subtopic)
	}

	w := watcher.New(&watcher.Config{
		Address:      1,
		Quantity:     64,
		RegisterSize: 2,
		SlaveID:      49,
		Read:         buildReader(handler, client),
	})

	err = w.Poll()
	if err != nil {
		fmt.Println(err)
		return
	}

	zones, err := getActiveZones(w)

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
		w.TriggerCallbacks()
	}

	mqttClient := MQTT.NewClient(connOpts)

	for _, zone := range zones {
		zone := zone
		zone.OnCurrentTempChange = func(currentTemp float32) {
			mqttClient.Publish(getTopic(zone.ZoneNumber, "currentTemperature"), 0, false, fmt.Sprintf("%g", currentTemp))
		}
		zone.OnTargetTempChange = func(targetTemp float32) {
			mqttClient.Publish(getTopic(zone.ZoneNumber, "targetTemperature"), 0, false, fmt.Sprintf("%g", targetTemp))
		}
		zone.OnFanModeChange = func(fanMode kn.FanMode) {
			fanModeStr, found := kn.FanModes.GetInverse(fanMode)
			if !found {
				log.Printf("Unknown fan mode %d", fanMode)
			}
			mqttClient.Publish(getTopic(zone.ZoneNumber, "fanMode"), 0, false, fanModeStr)
		}
		zone.OnHvacModeChange = func(hvacMode kn.HvacMode) {
			hvacModeStr, found := kn.HvacModes.GetInverse(hvacMode)
			if !found {
				log.Printf("Unknown hvac mode %d", hvacMode)
			}
			mqttClient.Publish(getTopic(zone.ZoneNumber, "hvacMode"), 0, false, hvacModeStr)
		}
	}

	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to %s\n", *server)
	}

	ticker := time.NewTicker(time.Second)

	go func() {
		for range ticker.C {
			err := w.Poll()
			if err != nil {
				log.Printf("Error polling modbus: %s\n", err)
			}
		}
	}()

	<-c

}
