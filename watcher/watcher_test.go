package watcher_test

import (
	"errors"
	"koolnova2mqtt/watcher"
	"testing"

	"github.com/epiclabs-io/ut"
)

func TestWatcher(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()

	var r []byte
	var readRegisterError error = nil
	readRegister := func(slaveID byte, address uint16, quantity uint16) (results []byte, err error) {
		return r, readRegisterError
	}

	w := watcher.New(&watcher.Config{
		Address:      1000,
		Quantity:     5,
		RegisterSize: 2,
		SlaveID:      1,
		Read:         readRegister,
	})

	var cbAddress uint16
	var callbackCount int
	w.RegisterCallback(1000, func(address uint16) {
		cbAddress = address
		callbackCount++
	})

	w.RegisterCallback(1004, func(address uint16) {
		callbackCount++
	})

	r = []byte{1, 2, 3, 4, 5, 6, 7, 8, 9, 10}

	value, err := w.ReadRegister(1001)
	t.MustFailWith(err, watcher.ErrUninitialized)
	t.Equals([]byte(nil), value)

	err = w.Poll()
	t.Ok(err)
	t.Equals(2, callbackCount)

	value, err = w.ReadRegister(1001)
	t.Ok(err)
	t.Equals([]byte{3, 4}, value)

	_, err = w.ReadRegister(200)
	t.MustFailWith(err, watcher.ErrAddressOutOfRange)

	_, err = w.ReadRegister(5000)
	t.MustFailWith(err, watcher.ErrAddressOutOfRange)

	callbackCount = 0
	err = w.Poll()
	t.Ok(err)

	t.Equals(callbackCount, 0)

	r = []byte{79, 82, 3, 4, 5, 6, 7, 8, 9, 10}
	callbackCount = 0
	err = w.Poll()
	t.Ok(err)

	t.Equals(1, callbackCount)
	t.Equals(uint16(1000), cbAddress)

	cbNewValue, err := w.ReadRegister(cbAddress)
	t.Ok(err)
	t.Equals([]byte{79, 82}, cbNewValue)

	r = []byte{1, 2}
	err = w.Poll()
	t.MustFailWith(err, watcher.ErrIncorrectRegisterSize)

	readRegisterError = errors.New("error")

	err = w.Poll()
	t.MustFail(err, "expected Poll() to fail if readRegister returns error")

}
