package watcher

import (
	"bytes"
	"errors"
	"koolnova2mqtt/modbus"
	"sync"
)

type Config struct {
	Address      uint16
	Quantity     uint16
	SlaveID      byte
	Modbus       modbus.Modbus
	RegisterSize int
}

type Watcher struct {
	Config
	state     []byte
	callbacks map[uint16]func(address uint16)
	lock      *sync.RWMutex
}

var ErrIncorrectRegisterSize = errors.New("Incorrect register size")
var ErrAddressOutOfRange = errors.New("Register address out of range")
var ErrUninitialized = errors.New("State uninitialized. Call Poll() first.")

func New(config *Config) *Watcher {
	return &Watcher{
		Config:    *config,
		callbacks: make(map[uint16]func(address uint16)),
		lock:      &sync.RWMutex{},
	}
}

func (w *Watcher) RegisterCallback(address uint16, callback func(address uint16)) {
	w.callbacks[address] = callback
}

func (w *Watcher) Poll() error {
	w.lock.Lock()
	newState, err := w.Modbus.ReadRegister(w.SlaveID, w.Address, w.Quantity)
	if err != nil {
		w.lock.Unlock()
		return err
	}

	if len(newState) != int(w.Quantity)*w.RegisterSize {
		w.lock.Unlock()
		return ErrIncorrectRegisterSize
	}

	oldState := w.state
	w.state = newState
	var callbackAddresses []uint16

	first := len(oldState) != len(newState)
	for n := 0; n < len(newState); n += w.RegisterSize {
		address := uint16(n/w.RegisterSize) + w.Address
		callback := w.callbacks[address]
		if callback == nil {
			continue
		}
		var oldValue []byte
		newValue := newState[n : n+w.RegisterSize]
		if first {
			oldValue = nil
		} else {
			oldValue = oldState[n : n+w.RegisterSize]
		}
		if bytes.Compare(oldValue, newValue) != 0 {
			callbackAddresses = append(callbackAddresses, address)
		}
	}
	w.lock.Unlock()
	for _, address := range callbackAddresses {
		callback := w.callbacks[address]
		callback(address)
	}
	return nil
}

func (w *Watcher) ReadRegister(address uint16) (value []byte) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if address < w.Address || address > w.Address+uint16(w.Quantity) {
		panic(ErrAddressOutOfRange)
	}
	if w.state == nil {
		panic(ErrUninitialized)
	}
	registerOffset := int(address-w.Address) * w.RegisterSize
	return w.state[registerOffset : registerOffset+w.RegisterSize]

}

func (w *Watcher) WriteRegister(address uint16, value uint16) error {
	w.lock.Lock()
	results, err := w.Modbus.WriteRegister(w.SlaveID, address, value)
	if err != nil {
		w.lock.Unlock()
		return err
	}
	registerOffset := int(address-w.Address) * w.RegisterSize
	copy(w.state[registerOffset:registerOffset+w.RegisterSize], results)
	callback := w.callbacks[address]
	w.lock.Unlock()
	if callback != nil {
		callback(address)
	}
	return nil
}

func (w *Watcher) TriggerCallbacks() {
	for address, callback := range w.callbacks {
		callback(address)
	}
}
