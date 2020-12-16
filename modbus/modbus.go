package modbus

import (
	"sync"
	"time"

	gmodbus "github.com/goburrow/modbus"
)

type Modbus interface {
	ReadRegister(slaveID byte, address uint16, quantity uint16) (results []byte, err error)
	WriteRegister(slaveID byte, address uint16, value uint16) (results []byte, err error)
	Close() error
}

type Config struct {
	Port     string
	BaudRate int
	DataBits int
	Parity   string
	StopBits int
	Timeout  time.Duration
}

type modbus struct {
	handler *gmodbus.RTUClientHandler
	client  gmodbus.Client
	lock    sync.RWMutex
}

func New(config *Config) (Modbus, error) {
	handler := gmodbus.NewRTUClientHandler(config.Port)
	handler.BaudRate = config.BaudRate
	handler.DataBits = config.DataBits
	handler.Parity = config.Parity
	handler.StopBits = config.StopBits
	handler.Timeout = config.Timeout

	return &modbus{
		handler: handler,
		client:  gmodbus.NewClient(handler),
	}, nil
}

func (mb *modbus) Close() error {
	return mb.handler.Close()
}

func (mb *modbus) ReadRegister(slaveID byte, address uint16, quantity uint16) (results []byte, err error) {
	mb.lock.Lock()
	defer mb.lock.Unlock()
	mb.handler.SlaveId = slaveID
	err = mb.handler.Connect()
	if err != nil {
		return nil, err
	}
	defer mb.handler.Close()
	results, err = mb.client.ReadHoldingRegisters(address-1, quantity)
	return results, err

}

func (mb *modbus) WriteRegister(slaveID byte, address uint16, value uint16) (results []byte, err error) {
	mb.lock.Lock()
	defer mb.lock.Unlock()
	mb.handler.SlaveId = slaveID
	err = mb.handler.Connect()
	if err != nil {
		return nil, err
	}
	defer mb.handler.Close()
	results, err = mb.client.WriteSingleRegister(address-1, value)
	return results, err

}
