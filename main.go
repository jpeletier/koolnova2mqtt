package main

import (
	"crypto/tls"
	"errors"
	"flag"
	"fmt"
	"koolnova2mqtt/kn"
	"koolnova2mqtt/modbus"
	"log"
	"os"
	"os/signal"
	"regexp"
	"strconv"
	"strings"
	"syscall"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

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

	mb, err := modbus.New(&modbus.Config{
		Port:     *modbusPort,
		BaudRate: *modbusPortBaudRate,
		DataBits: *modbusDataBits,
		Parity:   *modbusPortParity,
		StopBits: *modbusStopBits,
		Timeout:  5 * time.Second,
	})
	if err != nil {
		log.Fatalf("Error initializing modbus: %s", err)
	}
	defer mb.Close()

	var mqttClient MQTT.Client
	publish := func(topic string, qos byte, retained bool, payload string) {
		client := mqttClient
		if client == nil {
			log.Printf("Cannot publish message %q to topic %s. MQTT client is disconnected", payload, topic)
			return
		}
		client.Publish(topic, qos, retained, payload)
	}

	subscribe := func(topic string, callback func(message string)) error {
		client := mqttClient
		if client == nil {
			log.Printf("Cannot subscribe to topic %s. MQTT client is disconnected", topic)
			return errors.New("Client is disconnected")
		}
		token := client.Subscribe(topic, 0, func(c MQTT.Client, m MQTT.Message) {
			cbclient := mqttClient
			if cbclient != client {
				log.Printf("Cannot invoke callback to topic %s. MQTT client is disconnected", topic)
			}
			callback(string(m.Payload()))
		})
		token.Wait()
		return token.Error()
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
			ModuleName:  slaveName,
			SlaveID:     byte(slaveID),
			Publish:     publish,
			Subscribe:   subscribe,
			TopicPrefix: *prefix,
			HassPrefix:  *hassPrefix,
			Modbus:      mb,
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
	onConnect := false
	connOpts.OnConnect = func(c MQTT.Client) {
		onConnect = true
	}
	connOpts.OnConnectionLost = func(c MQTT.Client, err error) {
		log.Printf("Connection to MQTT server lost: %s\n", err)
		mqttClient = nil
	}

	connectMQTT := func() error {
		mqttClient = MQTT.NewClient(connOpts)

		if token := mqttClient.Connect(); token.Wait() && token.Error() != nil {
			mqttClient = nil
			return token.Error()
		} else {
			log.Printf("Connected to %s\n", *server)
		}
		return nil
	}

	ticker := time.NewTicker(time.Second)

	go func() {
		for range ticker.C {
			if mqttClient == nil {
				err := connectMQTT()
				if err != nil {
					log.Printf("Error connecting to MQTT server: %s\n", err)
					continue
				}
			}
			client := mqttClient
			if client != nil && client.IsConnected() {
				if onConnect {
					onConnect = false
					for _, b := range bridges {
						err := b.Start()
						if err != nil {
							log.Printf("Error starting bridge: %s\n", err)
							client.Disconnect(100)
							mqttClient = nil
						}
					}
				} else {
					for _, b := range bridges {
						b.Tick()
					}
				}
			}
		}
	}()

	<-ctrlC

}
