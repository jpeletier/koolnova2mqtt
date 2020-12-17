package modbus

import (
	"log"
	"sync"
	"time"

	gmodbus "github.com/wz2b/modbus"
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

func throttle(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
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
	}, handler.Connect()
}

func (mb *modbus) Close() error {
	return mb.handler.Close()
}

func (mb *modbus) ReadRegister(slaveID byte, address uint16, quantity uint16) (results []byte, err error) {
	err = mb.try(slaveID, func() (err error) {
		results, err = mb.client.ReadHoldingRegisters(address-1, quantity)
		return err
	})
	return results, err
}

func (mb *modbus) WriteRegister(slaveID byte, address uint16, value uint16) (results []byte, err error) {
	err = mb.try(slaveID, func() (err error) {
		results, err = mb.client.WriteSingleRegister(address-1, value)
		return err
	})
	return results, err
}

func (mb *modbus) try(slaveID byte, f func() error) (err error) {
	mb.lock.Lock()
	defer mb.lock.Unlock()
	defer throttle(100)
	mb.handler.SlaveId = slaveID
	retries := 5
	delay := 100
	for retries > 0 {
		err = f()
		if err == nil {
			return nil
		}
		log.Printf("Retried modbus operation due to %s. %d retries left\n", err, retries)
		mb.handler.Close()
		throttle(100)
		err = mb.handler.Connect()
		if err != nil {
			return err
		}
		retries--
		throttle(delay)
		delay *= 2
	}
	return err
}
