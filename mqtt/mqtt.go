package mqtt

import (
	"crypto/tls"
	"errors"
	"log"
	"time"

	MQTT "github.com/eclipse/paho.mqtt.golang"
)

type Config struct {
	Server   string
	ClientID string
	Username string
	Password string
}

type Client struct {
	client MQTT.Client
	ID     int
	closed bool
}

var ErrNotConnected = errors.New("MQTT client not connected")

func New(config *Config) *Client {
	m := &Client{}

	connOpts := MQTT.NewClientOptions().
		AddBroker(config.Server).
		SetClientID(config.ClientID).
		SetCleanSession(true).
		SetAutoReconnect(false)

	if config.Username != "" {
		connOpts.SetUsername(config.Username)
		if config.Password != "" {
			connOpts.SetPassword(config.Password)
		}
	}

	tlsConfig := &tls.Config{InsecureSkipVerify: true, ClientAuth: tls.NoClientCert}
	connOpts.SetTLSConfig(tlsConfig)

	connOpts.OnConnectionLost = func(c MQTT.Client, err error) {
		log.Printf("MQTT disconnected: %s\n", err)
	}

	connect := func() {
		log.Printf("Trying to connect to MQTT %s ...\n", config.Server)
		newClient := MQTT.NewClient(connOpts)
		token := newClient.Connect()
		token.Wait()
		if token.Error() == nil {
			m.client = newClient
			m.ID++
			log.Printf("Connected to MQTT. Session ID %d\n", m.ID)
		}
	}

	connect()
	go func() {
		ticker := time.NewTicker(5 * time.Second)
		for range ticker.C {
			if m.closed {
				return
			}
			if m.client == nil || !m.client.IsConnectionOpen() {
				connect()
			}
		}
		if m.client != nil {
			m.client.Disconnect(100)
		}
	}()
	return m
}

func (m *Client) Publish(topic string, qos byte, retained bool, payload string) error {
	if m.client == nil {
		return ErrNotConnected
	}
	token := m.client.Publish(topic, qos, retained, payload)
	token.Wait()
	return token.Error()
}

func (m *Client) Subscribe(topic string, callback func(message string)) error {
	if m.client == nil {
		return ErrNotConnected
	}
	token := m.client.Subscribe(topic, 0, func(c MQTT.Client, m MQTT.Message) {
		callback(string(m.Payload()))
	})
	token.Wait()
	return token.Error()
}

func (m *Client) Close() error {
	m.closed = true
	return nil
}
