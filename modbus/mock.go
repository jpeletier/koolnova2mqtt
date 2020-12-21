package modbus

import (
	"errors"
)

type Mock struct {
	State map[byte][]uint16
}

func NewMock() *Mock {
	return &Mock{
		State: map[byte][]uint16{
			49: {3, 68, 41, 41, 3, 68, 41, 41, 3, 68, 41, 45, 3, 68, 41, 45, 3, 68, 41, 42, 3, 52, 41, 40, 3, 52, 41, 44, 3, 68, 41, 41, 3, 68, 41, 40, 3, 68, 41, 41, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 4, 3, 4, 2, 49, 3, 7, 1, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
			50: {3, 68, 41, 41, 3, 68, 41, 41, 3, 68, 41, 45, 3, 68, 41, 45, 3, 68, 41, 42, 3, 52, 41, 40, 3, 52, 41, 44, 3, 68, 41, 41, 3, 68, 41, 40, 3, 68, 41, 41, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 68, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 4, 4, 3, 4, 2, 49, 3, 7, 1, 4, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0, 0},
		},
	}
}

func (ms *Mock) ReadRegister(slaveID byte, address uint16, quantity uint16) (results []uint16, err error) {
	state, ok := ms.State[slaveID]
	if !ok {
		return nil, errors.New("Unknown slave")
	}
	address--
	for a := address; a < address+quantity; a++ {
		results = append(results, state[a])
	}
	return results, nil
}
func (ms *Mock) WriteRegister(slaveID byte, address uint16, value uint16) (results []uint16, err error) {
	state, ok := ms.State[slaveID]
	if !ok {
		return nil, errors.New("Unknown slave")
	}
	state[address-1] = value
	return []uint16{value}, nil
}

func (ms *Mock) Close() error { return nil }
