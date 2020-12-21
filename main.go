package main

import (
	"koolnova2mqtt/kn"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

// newBridges builds all bridges from a list of Modbus slaves
func newBridges(slaves map[byte]string, templateConfig *kn.Config) []*kn.Bridge {
	var bridges []*kn.Bridge
	for id, name := range slaves {
		config := *templateConfig
		config.ModuleName = name
		config.SlaveID = id
		bridge := kn.NewBridge(&config)
		bridges = append(bridges, bridge)
	}
	return bridges
}

func main() {

	// configure CTRL+C as a way to stop the application
	ctrlC := make(chan os.Signal, 1)
	signal.Notify(ctrlC, os.Interrupt, syscall.SIGTERM)

	// read configuration from the command line
	config := ParseCommandLine()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		var sessionID int
		var bridges []*kn.Bridge
		for range ticker.C {
			newSessionID := config.MqttClient.ID
			if sessionID != newSessionID {
				bridges = newBridges(config.slaves, config.BridgeTemplateConfig)
				for _, b := range bridges {
					err := b.Start()
					if err != nil {
						log.Printf("Error starting bridge: %s\n", err)
						break
					} else {
						sessionID = newSessionID
					}
				}
			} else {
				for _, b := range bridges {
					b.Tick()
				}
			}
		}
	}()

	<-ctrlC

	config.MqttClient.Close()
	config.BridgeTemplateConfig.Modbus.Close()

}
