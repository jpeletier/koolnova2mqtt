package main

import (
	"flag"
	"fmt"
	"koolnova2mqtt/kn"
	"koolnova2mqtt/modbus"
	"koolnova2mqtt/mqtt"
	"log"
	"os"
	"regexp"
	"strconv"
	"strings"
	"time"
)

type Config struct {
	MqttClient           *mqtt.Client
	slaves               map[byte]string
	BridgeTemplateConfig *kn.Config
}

func generateNodeName(slaveID string, port string) string {
	reg, err := regexp.Compile("[^a-zA-Z0-9]+")
	if err != nil {
		log.Fatal(err)
	}
	hostname, _ := os.Hostname()

	port = strings.Replace(port, "/dev/", "", -1)
	port = reg.ReplaceAllString(port, "")
	return strings.ToLower(fmt.Sprintf("%s_%s_%s", hostname, port, slaveID))

}

func parseModbusSlaveInfo(slaveIDs, slaveNames string, modbusPort string) map[byte]string {
	slaveIDStrList := strings.Split(slaveIDs, ",")
	var slaveNameList []string

	if slaveNames == "" {
		for _, slaveIDStr := range slaveIDStrList {
			slaveNameList = append(slaveNameList, generateNodeName(slaveIDStr, modbusPort))
		}
	} else {
		slaveNameList = strings.Split(slaveNames, ",")
		if len(slaveIDStrList) != len(slaveNameList) {
			log.Fatalf("modbusSlaveIDs and modbusSlaveNames lists must have the same length")
		}
	}

	slaves := make(map[byte]string)
	for i, slaveIDStr := range slaveIDStrList {
		slaveID, err := strconv.Atoi(slaveIDStr)
		if err != nil {
			log.Fatalf("Error parsing slaveID list")
		}
		slaves[byte(slaveID)] = slaveNameList[i]
	}
	return slaves
}

func ParseCommandLine() *Config {
	hostname, _ := os.Hostname()

	server := flag.String("server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	clientid := flag.String("clientid", hostname+strconv.Itoa(time.Now().Second()), "A clientid for the connection")
	username := flag.String("username", "", "A username to authenticate to the MQTT server")
	password := flag.String("password", "", "Password to match username")
	prefix := flag.String("prefix", "koolnova2mqtt", "MQTT topic root where to publish/read topics")
	hassPrefix := flag.String("hassPrefix", "homeassistant", "Home assistant discovery prefix")
	modbusPort := flag.String("modbusPort", "/dev/ttyUSB0", "Serial port where modbus hardware is connected")
	modbusPortBaudRate := flag.Int("modbusRate", 9600, "Modbus port data rate")
	modbusDataBits := flag.Int("modbusDataBits", 8, "Modbus port data bits")
	modbusPortParity := flag.String("modbusParity", "E", "N - None, E - Even, O - Odd (default E) (The use of no parity requires 2 stop bits.)")
	modbusStopBits := flag.Int("modbusStopBits", 1, "Modbus port stop bits")
	modbusSlaveList := flag.String("modbusSlaveIDs", "49", "Comma-separated list of modbus slave IDs to manage")
	modbusSlaveNames := flag.String("modbusSlaveNames", "", "Comma-separated list of modbus slave names. Defaults to 'slave#'")

	flag.Parse()

	slaves := parseModbusSlaveInfo(*modbusSlaveList, *modbusSlaveNames, *modbusPort)

	mb, err := modbus.New(&modbus.Config{
		Port:     *modbusPort,
		BaudRate: *modbusPortBaudRate,
		DataBits: *modbusDataBits,
		Parity:   *modbusPortParity,
		StopBits: *modbusStopBits,
		Timeout:  200 * time.Millisecond,
	})
	if err != nil {
		log.Fatalf("Error initializing modbus: %s", err)
	}
	defer mb.Close()

	mqttClient := mqtt.New(&mqtt.Config{
		Server:   *server,
		ClientID: *clientid,
		Username: *username,
		Password: *password,
	})

	return &Config{
		slaves:     slaves,
		MqttClient: mqttClient,
		BridgeTemplateConfig: &kn.Config{
			Mqtt:        mqttClient,
			Modbus:      mb,
			TopicPrefix: *prefix,
			HassPrefix:  *hassPrefix,
		},
	}

}
