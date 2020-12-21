package modbus

import (
	"encoding/binary"
	"errors"
	"log"
	"sync"
	"time"

	gmodbus "github.com/wz2b/modbus"
)

type Config struct {
	Port     string
	BaudRate int
	DataBits int
	Parity   string
	StopBits int
	Timeout  time.Duration
}

type Modbus struct {
	handler *gmodbus.RTUClientHandler
	client  gmodbus.Client
	lock    sync.RWMutex
}

func throttle(ms int) {
	time.Sleep(time.Duration(ms) * time.Millisecond)
}

var ErrIncorrectResultSize = errors.New("Incorrect number of results returned")

func New(config *Config) (*Modbus, error) {
	handler := gmodbus.NewRTUClientHandler(config.Port)
	handler.BaudRate = config.BaudRate
	handler.DataBits = config.DataBits
	handler.Parity = config.Parity
	handler.StopBits = config.StopBits
	handler.Timeout = config.Timeout

	return &Modbus{
		handler: handler,
		client:  gmodbus.NewClient(handler),
	}, handler.Connect()
}

func (mb *Modbus) Close() error {
	return mb.handler.Close()
}

func parseResults(r []byte, quantity uint16) ([]uint16, error) {
	if len(r) != int(quantity*2) {
		return nil, ErrIncorrectResultSize
	}
	results := make([]uint16, quantity)
	for n := uint16(0); n < quantity; n++ {
		results[n] = binary.BigEndian.Uint16(r[n*2 : n*2+2])
	}
	return results, nil
}

func (mb *Modbus) ReadRegister(slaveID byte, address uint16, quantity uint16) (results []uint16, err error) {
	err = mb.try(slaveID, func() (err error) {
		r, err := mb.client.ReadHoldingRegisters(address-1, quantity)
		if err != nil {
			return err
		}
		results, err = parseResults(r, quantity)
		return err
	})
	return results, err
}

func (mb *Modbus) WriteRegister(slaveID byte, address uint16, value uint16) (results []uint16, err error) {
	err = mb.try(slaveID, func() (err error) {
		r, err := mb.client.WriteSingleRegister(address-1, value)
		if err != nil {
			return err
		}
		results, err = parseResults(r, 1)
		return err
	})
	return results, err
}

func (mb *Modbus) try(slaveID byte, f func() error) (err error) {
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
		connectErr := mb.handler.Connect()
		if connectErr != nil {
			return connectErr
		}
		retries--
		throttle(delay)
		delay *= 2
	}
	return err
}
