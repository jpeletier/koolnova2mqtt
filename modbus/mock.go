package modbus

import (
	"encoding/binary"
	"errors"
)

type Mock struct {
	State map[byte][]uint16
}

func NewMock() Modbus {
	return &Mock{
		State: map[byte][]uint16{
			49: {3, 68, 41, 41, 3, 68, 41, 41, 3, 68, 41, 45, 3, 68, 41, 45, 3, 68, 41, 42, 3, 52, 41, 40, 3, 52, 41, 44, 3, 68, 41, 41, 3, 68, 41, 40, 3, 68, 41, 41, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0},
			50: {3, 68, 41, 41, 3, 68, 41, 41, 3, 68, 41, 45, 3, 68, 41, 45, 3, 68, 41, 42, 3, 52, 41, 40, 3, 52, 41, 44, 3, 68, 41, 41, 3, 68, 41, 40, 3, 68, 41, 41, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0},
		},
	}
}

func (ms *Mock) ReadRegister(slaveID byte, address uint16, quantity uint16) (results []byte, err error) {
	state, ok := ms.State[slaveID]
	if !ok {
		return nil, errors.New("Unknown slave")
	}
	for a := address; a < address+quantity; a++ {
		b := make([]byte, 2)
		binary.BigEndian.PutUint16(b, state[a])
		results = append(results, b...)
	}
	return results, nil
}
func (ms *Mock) WriteRegister(slaveID byte, address uint16, value uint16) (results []byte, err error) {
	state, ok := ms.State[slaveID]
	if !ok {
		return nil, errors.New("Unknown slave")
	}
	state[address] = value
	results = make([]byte, 2)
	binary.BigEndian.PutUint16(results, value)
	return results, nil
}

func (ms *Mock) Close() error { return nil }
