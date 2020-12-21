package kn_test

import (
	"encoding/json"
	"fmt"
	"koolnova2mqtt/kn"
	"koolnova2mqtt/modbus"
	"sort"
	"strconv"
	"strings"
	"testing"

	"github.com/epiclabs-io/ut"
)

type Message struct {
	Topic   string
	Payload interface{}
}

type MqttClientMock struct {
	subscriptions map[string]func(message string)
	messages      []Message
}

func NewMqttClientMock() *MqttClientMock {
	return &MqttClientMock{
		subscriptions: make(map[string]func(string)),
	}
}

func (m *MqttClientMock) Publish(topic string, qos byte, retained bool, payload string) error {
	var jsonObject map[string]interface{}
	var p interface{}
	err := json.Unmarshal([]byte(payload), &jsonObject)
	if err == nil {
		p = jsonObject
	} else {
		p = payload
	}

	m.messages = append(m.messages, Message{
		Topic:   topic,
		Payload: p,
	})
	return nil
}

func (m *MqttClientMock) simulateMessage(topic string, payload string) {
	callback := m.subscriptions[topic]
	if callback != nil {
		callback(payload)
	}
}

func (m *MqttClientMock) LastMessage() *Message {
	if len(m.messages) == 0 {
		return nil
	}
	return &m.messages[len(m.messages)-1]
}

func (m *MqttClientMock) Clear() {
	m.messages = nil
}

func (m *MqttClientMock) Subscribe(topic string, callback func(message string)) error {
	m.subscriptions[topic] = callback
	return nil
}

func getKeys(funcMap map[string]func(string)) []string {
	keys := make([]string, 0, len(funcMap))
	for k := range funcMap {
		keys = append(keys, k)
	}
	sort.Slice(keys, func(i, j int) bool {
		return strings.Compare(keys[i], keys[j]) < 0
	})
	return keys
}

type DiffValue struct {
	Address uint16
	Old     uint16
	New     uint16
}

type TestMessage struct {
	ID      int
	Topic   string
	Payload string
	Diffs   []DiffValue
}

func diffState(old, new []uint16) []DiffValue {
	if len(old) != len(new) {
		panic("not the same length")
	}
	var diffs []DiffValue
	for n := 0; n < len(old); n++ {
		if old[n] != new[n] {
			diffs = append(diffs, DiffValue{
				Old:     old[n],
				New:     new[n],
				Address: uint16(n + 1),
			})
		}
	}
	return diffs
}

func TestBridge(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	var err error

	mqttClient := NewMqttClientMock()
	modbusClient := modbus.NewMock()
	b := kn.NewBridge(&kn.Config{
		ModuleName:  "TestModule",
		SlaveID:     49,
		TopicPrefix: "topicPrefix",
		HassPrefix:  "hassPrefix",
		Mqtt:        mqttClient,
		Modbus:      modbusClient,
	})

	// Check the correct subscriptions and messages are sent on connect:
	err = b.Start()
	t.Ok(err)
	t.EqualsFile("subscriptions.json", getKeys(mqttClient.subscriptions))
	sort.Slice(mqttClient.messages, func(i, j int) bool {
		return strings.Compare(mqttClient.messages[i].Topic, mqttClient.messages[j].Topic) < 0
	})
	t.EqualsFile("connect-messages.json", mqttClient.messages)
	mqttClient.Clear()

	// Check modbus state is altered when various control messages are received over MQTT

	var messages []TestMessage
	simulateMessage := func(topic, payload string) {
		state := append([]uint16(nil), modbusClient.State[49]...)
		mqttClient.simulateMessage(topic, payload)
		messages = append(messages, TestMessage{
			ID:      len(messages) + 1,
			Topic:   topic,
			Payload: payload,
			Diffs:   diffState(state, modbusClient.State[49]),
		})
	}
	simulateMessage("topicPrefix/TestModule/sys/holdMode/set", kn.HOLD_MODE_FAN_ONLY)
	simulateMessage("topicPrefix/TestModule/sys/holdMode/set", kn.HOLD_MODE_UNDERFLOOR_AND_FAN)
	simulateMessage("topicPrefix/TestModule/sys/holdMode/set", kn.HOLD_MODE_UNDERFLOOR_ONLY)
	simulateMessage("topicPrefix/TestModule/sys/holdMode/set", "bad mode")

	hvacModes := []string{kn.HVAC_MODE_COOL, kn.HVAC_MODE_HEAT, kn.HVAC_MODE_OFF}
	holdModes := []string{kn.HOLD_MODE_UNDERFLOOR_ONLY, kn.HOLD_MODE_FAN_ONLY, kn.HOLD_MODE_UNDERFLOOR_AND_FAN}

	for z := 1; z < 11; z++ {
		simulateMessage(fmt.Sprintf("topicPrefix/TestModule/zone%d/fanMode/set", z), kn.FanMode2Str(kn.FAN_HIGH))
		simulateMessage(fmt.Sprintf("topicPrefix/TestModule/zone%d/fanMode/set", z), kn.FanMode2Str(kn.FAN_MED))
		simulateMessage(fmt.Sprintf("topicPrefix/TestModule/zone%d/fanMode/set", z), kn.FanMode2Str(kn.FAN_LOW))
		simulateMessage(fmt.Sprintf("topicPrefix/TestModule/zone%d/fanMode/set", z), kn.FanMode2Str(kn.FAN_AUTO))
		simulateMessage(fmt.Sprintf("topicPrefix/TestModule/zone%d/fanMode/set", z), "bad mode")
		simulateMessage(fmt.Sprintf("topicPrefix/TestModule/zone%d/targetTemp/set", z), strconv.Itoa(20+z))
		for i := 0; i < len(hvacModes); i++ {
			simulateMessage(fmt.Sprintf("topicPrefix/TestModule/zone%d/hvacMode/set", z), hvacModes[i])
			for j := 0; j < len(holdModes); j++ {
				simulateMessage("topicPrefix/TestModule/sys/holdMode/set", holdModes[j])
			}
		}
	}

	// diffs.json will contain a list of changes. Each item in the array is the result
	// of each simulateMessage call above.
	t.EqualsFile("diffs.json", messages)
	t.EqualsFile("messages.json", mqttClient.messages)

	// simulate changes in temperature by writing random values to temperature registers
	mqttClient.Clear()
	n := 0
	for i := 0; i < 20; i++ {
		for z := 1; z < 11; z++ {
			t := (i + 15) * 2
			n++
			modbusClient.WriteRegister(49, uint16((z-1)*kn.REG_PER_ZONE+kn.REG_CURRENT_TEMP), uint16(t))
		}
		b.Tick()
	}
	t.EqualsFile("current-temp.json", mqttClient.messages)

}
