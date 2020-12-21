package watcher_test

import (
	"errors"
	"koolnova2mqtt/modbus"
	"koolnova2mqtt/watcher"
	"testing"

	"github.com/epiclabs-io/ut"
)

var modbusError error

type BuggyModbus struct {
}

func (ms *BuggyModbus) ReadRegister(slaveID byte, address uint16, quantity uint16) (results []uint16, err error) {
	return []uint16{0x0102}, modbusError

}
func (ms *BuggyModbus) WriteRegister(slaveID byte, address uint16, value uint16) (results []uint16, err error) {
	return nil, modbusError
}

func (ms *BuggyModbus) Close() error { return nil }

func TestWatcher(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	var err error

	w := watcher.New(&watcher.Config{
		Address:  1,
		Quantity: 5,
		SlaveID:  49,
		Modbus:   modbus.NewMock(),
	})

	var cbAddress uint16
	var callbackCount int
	w.RegisterCallback(3, func(address uint16) {
		cbAddress = address
		callbackCount++
	})

	w.RegisterCallback(4, func(address uint16) {
		callbackCount++
	})

	t.MustPanicWith(watcher.ErrUninitialized, func() {
		w.ReadRegister(1)
	})

	err = w.Poll()
	t.Ok(err)
	t.Equals(2, callbackCount)

	value := w.ReadRegister(1)
	t.Ok(err)
	t.Equals(uint16(0x0003), value)

	t.MustPanicWith(watcher.ErrAddressOutOfRange, func() {
		w.ReadRegister(200)
	})

	t.MustPanicWith(watcher.ErrAddressOutOfRange, func() {
		w.ReadRegister(5000)
	})

	callbackCount = 0
	err = w.Poll()
	t.Ok(err)
	t.Equals(callbackCount, 0)

	callbackCount = 0
	err = w.WriteRegister(3, 0x1234)
	t.Ok(err)
	t.Equals(1, callbackCount)
	t.Equals(uint16(3), cbAddress)

	cbNewValue := w.ReadRegister(3)
	t.Equals(uint16(0x1234), cbNewValue)

	callbackCount = 0
	err = w.Poll()
	t.Ok(err)
	t.Equals(0, callbackCount)

	w.TriggerCallbacks()
	t.Equals(2, callbackCount)

	w = watcher.New(&watcher.Config{
		Address:  1,
		Quantity: 5,
		SlaveID:  1,
		Modbus:   &BuggyModbus{},
	})

	modbusError = errors.New("error")

	err = w.Poll()
	t.MustFail(err, "expected Poll() to fail if readRegister returns error")

}
