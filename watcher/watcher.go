// Package watcher represents a cache of a range of modbus registers in a device
// Can fire events if a watched register changes
package watcher

import (
	"errors"
	"koolnova2mqtt/modbus"
	"sync"
)

// Config contains the configuration parameters for a new Watcher instance
type Config struct {
	Address  uint16        // Start address
	Quantity uint16        // Number of registers to watch
	SlaveID  byte          // SlaveID to watch
	Modbus   modbus.Modbus // Modbus interface
}

// Watcher represents a cache of modbus registers in a device
type Watcher struct {
	Config
	state     []uint16                        // current view of the modbus register states
	callbacks map[uint16]func(address uint16) // set of callbacks
	lock      *sync.RWMutex
}

var ErrAddressOutOfRange = errors.New("Register address out of range")
var ErrUninitialized = errors.New("State uninitialized. Call Poll() first.")
var ErrCannotIncreaseRange = errors.New("Cannot increase range")

// New returns a new Watcher instance
func New(config *Config) *Watcher {
	return &Watcher{
		Config:    *config,
		callbacks: make(map[uint16]func(address uint16)),
		lock:      &sync.RWMutex{},
	}
}

// RegisterCallback registers a new callback that will be fired when the specific register address changes values
func (w *Watcher) RegisterCallback(address uint16, callback func(address uint16)) {
	w.callbacks[address] = callback
}

// Poll refreshes the cache by reading the watched register range from the slave device
func (w *Watcher) Poll() error {
	w.lock.Lock()
	newState, err := w.Modbus.ReadRegister(w.SlaveID, w.Address, w.Quantity)
	if err != nil {
		w.lock.Unlock()
		return err
	}

	oldState := w.state
	w.state = newState
	var callbackAddresses []uint16

	first := len(oldState) != len(newState)
	for n := 0; n < len(newState); n++ {
		address := uint16(n) + w.Address
		callback := w.callbacks[address]
		if callback == nil {
			continue
		}
		if first || oldState[n] != newState[n] {
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

// ReadRegister reads one register from the cache
func (w *Watcher) ReadRegister(address uint16) (value uint16) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if address < w.Address || address > w.Address+uint16(w.Quantity) {
		panic(ErrAddressOutOfRange)
	}
	if w.state == nil {
		panic(ErrUninitialized)
	}
	return w.state[int(address-w.Address)]

}

// WriteRegister writes the value to the slave device and updates the cache if successful
func (w *Watcher) WriteRegister(address uint16, value uint16) error {
	w.lock.Lock()
	results, err := w.Modbus.WriteRegister(w.SlaveID, address, value)
	if err != nil {
		w.lock.Unlock()
		return err
	}
	w.state[int(address-w.Address)] = results[0]
	callback := w.callbacks[address]
	w.lock.Unlock()
	if callback != nil {
		callback(address)
	}
	return nil
}

// TriggerCallbacks calls all callbacks
func (w *Watcher) TriggerCallbacks() {
	for address, callback := range w.callbacks {
		callback(address)
	}
}

// Resize reduces the watched range
func (w *Watcher) Resize(newQuantity int) {
	w.lock.Lock()
	defer w.lock.Unlock()
	if newQuantity <= int(w.Quantity) {
		w.state = w.state[:newQuantity]
		for address := range w.callbacks {
			if address > w.Address+uint16(newQuantity)-1 {
				delete(w.callbacks, address)
			}
		}
	} else {
		panic(ErrCannotIncreaseRange)
	}
	w.Quantity = uint16(newQuantity)
}
