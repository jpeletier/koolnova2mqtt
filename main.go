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
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
	"github.com/goburrow/modbus"
)

func buildReader(handler *modbus.RTUClientHandler, client modbus.Client) watcher.ReadRegister {
	return func(slaveID byte, address uint16, quantity uint16) (results []byte, err error) {
		handler.SlaveId = slaveID
		results, err = client.ReadHoldingRegisters(address-1, quantity)
		return results, err
	}
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

func main() {

	ctrlC := make(chan os.Signal, 1)
	signal.Notify(ctrlC, os.Interrupt, syscall.SIGTERM)

	hostname, _ := os.Hostname()

	server := flag.String("server", "tcp://127.0.0.1:1883", "The full url of the MQTT server to connect to ex: tcp://127.0.0.1:1883")
	//topic := flag.String("topic", "#", "Topic to subscribe to")
	//qos := flag.Int("qos", 0, "The QoS to subscribe to messages at")
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

	handler := modbus.NewRTUClientHandler(*modbusPort)
	handler.BaudRate = *modbusPortBaudRate
	handler.DataBits = *modbusDataBits
	handler.Parity = *modbusPortParity
	handler.StopBits = *modbusStopBits
	handler.Timeout = 5 * time.Second

	err := handler.Connect()
	if err != nil {
		fmt.Println(err)
	}
	defer handler.Close()

	modbusClient := modbus.NewClient(handler)

	registerReader := buildReader(handler, modbusClient)

	var mqttClient MQTT.Client
	publish := func(topic string, qos byte, retained bool, payload string) {
		mqttClient.Publish(topic, qos, retained, payload)
	}

	var snameList []string
	slist := strings.Split(*modbusSlaveList, ",")

	if *modbusSlaveNames == "" {
		for _, slaveIDStr := range slist {
			snameList = append(snameList, generateNodeName(slaveIDStr, *modbusPort))
		}
	} else {
		snameList = strings.Split(*modbusSlaveNames, ",")
		if len(slist) != len(snameList) {
			log.Fatalf("modbusSlaveIDs and modbusSlaveNames lists must have the same length")
		}
	}

	var bridges []*kn.Bridge
	for i, slaveIDStr := range slist {
		slaveID, err := strconv.Atoi(slaveIDStr)
		slaveName := snameList[i]
		if err != nil {
			log.Fatalf("Error parsing slaveID list")
		}
		bridge := kn.NewBridge(&kn.Config{
			ModuleName:   slaveName,
			SlaveID:      byte(slaveID),
			Publish:      publish,
			TopicPrefix:  *prefix,
			HassPrefix:   *hassPrefix,
			ReadRegister: registerReader,
		})
		bridges = append(bridges, bridge)
	}

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
		for _, b := range bridges {
			b.Start()
		}
	}

	mqttClient = MQTT.NewClient(connOpts)

	if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
		panic(token.Error())
	} else {
		fmt.Printf("Connected to %s\n", *server)
	}

	ticker := time.NewTicker(time.Second)

	go func() {
		for range ticker.C {
			for _, b := range bridges {
				b.Tick()
			}
		}
	}()

	<-ctrlC

}
