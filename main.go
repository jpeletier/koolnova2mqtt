package main

import (
	"koolnova2mqtt/kn"
	"log"
	"os"
	"os/signal"
	"syscall"
	"time"
)

func NewBridges(slaves map[byte]string, templateConfig *kn.Config) []*kn.Bridge {
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

	ctrlC := make(chan os.Signal, 1)
	signal.Notify(ctrlC, os.Interrupt, syscall.SIGTERM)

	config := ParseCommandLine()

	go func() {
		ticker := time.NewTicker(2 * time.Second)
		var sessionID int
		var bridges []*kn.Bridge
		for range ticker.C {
			newSessionID := config.MqttClient.ID
			if sessionID != newSessionID {
				bridges = NewBridges(config.slaves, config.BridgeTemplateConfig)
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

}
