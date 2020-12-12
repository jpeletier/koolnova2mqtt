package bimap

import (
	"reflect"
	"runtime/debug"
	"testing"

	"github.com/epiclabs-io/ut"
)

const key = "key"
const value = "value"

// isEmpty gets whether the specified object is considered empty or not.
func isEmpty(object interface{}) bool {

	// get nil case out of the way
	if object == nil {
		return true
	}

	objValue := reflect.ValueOf(object)

	switch objValue.Kind() {
	// collection types are empty when they have no element
	case reflect.Array, reflect.Chan, reflect.Map, reflect.Slice:
		return objValue.Len() == 0
		// pointers are empty if nil or if the value they point to is empty
	case reflect.Ptr:
		if objValue.IsNil() {
			return true
		}
		deref := objValue.Elem().Interface()
		return isEmpty(deref)
		// for all other types, compare against the zero value
	default:
		zero := reflect.Zero(objValue.Type())
		return reflect.DeepEqual(object, zero.Interface())
	}
}

// didPanic returns true if the function passed to it panics. Otherwise, it returns false.
func didPanic(f func()) (bool, interface{}, string) {

	didPanic := false
	var message interface{}
	var stack string
	func() {

		defer func() {
			if message = recover(); message != nil {
				didPanic = true
				stack = string(debug.Stack())
			}
		}()

		// call the target function
		f()

	}()

	return didPanic, message, stack

}

func TestNewBiMap(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()

	actual := NewBiMap()
	expected := &BiMap{forward: make(map[interface{}]interface{}), inverse: make(map[interface{}]interface{})}
	t.Equals(expected, actual)
}

func TestBiMap_Insert(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()
	actual.Insert(key, value)

	fwdExpected := make(map[interface{}]interface{})
	invExpected := make(map[interface{}]interface{})
	fwdExpected[key] = value
	invExpected[value] = key
	expected := &BiMap{forward: fwdExpected, inverse: invExpected}

	t.Equals(expected, actual)
}

func TestBiMap_Exists(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()

	actual.Insert(key, value)
	t.Assert(!actual.Exists("ARBITARY_KEY"), "Key should not exist")
	t.Assert(actual.Exists(key), "Inserted key should exist")
}

func TestBiMap_InverseExists(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()

	actual.Insert(key, value)
	t.Assert(!actual.ExistsInverse("ARBITARY_VALUE"), "Value should not exist")
	t.Assert(actual.ExistsInverse(value), "Inserted value should exist")
}

func TestBiMap_Get(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()

	actual.Insert(key, value)

	actualVal, ok := actual.Get(key)

	t.Assert(ok, "It should return true")
	t.Equals(value, actualVal)

	actualVal, ok = actual.Get(value)

	t.Assert(!ok, "It should return false")
	t.Assert(isEmpty(actualVal), "Actual val should be empty")
}

func TestBiMap_GetInverse(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()

	actual.Insert(key, value)

	actualKey, ok := actual.GetInverse(value)

	t.Assert(ok, "It should return true")
	t.Equals(key, actualKey)

	actualKey, ok = actual.Get(value)

	t.Assert(!ok, "It should return false")
	t.Assert(isEmpty(actualKey), "Actual key should be empty")
}

func TestBiMap_Size(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()

	t.Equals(0, actual.Size())

	actual.Insert(key, value)

	t.Equals(1, actual.Size())
}

func TestBiMap_Delete(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()
	dummyKey := "DummyKey"
	dummyVal := "DummyVal"
	actual.Insert(key, value)
	actual.Insert(dummyKey, dummyVal)

	t.Equals(2, actual.Size())

	actual.Delete(dummyKey)

	fwdExpected := make(map[interface{}]interface{})
	invExpected := make(map[interface{}]interface{})
	fwdExpected[key] = value
	invExpected[value] = key

	expected := &BiMap{forward: fwdExpected, inverse: invExpected}

	t.Equals(1, actual.Size())
	t.Equals(expected, actual)

	actual.Delete(dummyKey)

	t.Equals(1, actual.Size())
	t.Equals(expected, actual)
}

func TestBiMap_InverseDelete(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()
	dummyKey := "DummyKey"
	dummyVal := "DummyVal"
	actual.Insert(key, value)
	actual.Insert(dummyKey, dummyVal)

	t.Equals(2, actual.Size())

	actual.DeleteInverse(dummyVal)

	fwdExpected := make(map[interface{}]interface{})
	invExpected := make(map[interface{}]interface{})
	fwdExpected[key] = value
	invExpected[value] = key

	expected := &BiMap{forward: fwdExpected, inverse: invExpected}

	t.Equals(1, actual.Size())
	t.Equals(expected, actual)

	actual.DeleteInverse(dummyVal)

	t.Equals(1, actual.Size())
	t.Equals(expected, actual)
}

func TestBiMap_WithVaryingType(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()
	dummyKey := "Dummy key"
	dummyVal := 3

	actual.Insert(dummyKey, dummyVal)

	res, _ := actual.Get(dummyKey)
	resVal, _ := actual.GetInverse(dummyVal)
	t.Equals(dummyVal, res)
	t.Equals(dummyKey, resVal)

}

func TestBiMap_MakeImmutable(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()
	dummyKey := "Dummy key"
	dummyVal := 3

	actual.Insert(dummyKey, dummyVal)

	actual.MakeImmutable()

	panicked, _, _ := didPanic(func() {
		actual.Delete(dummyKey)
	})
	t.Assert(panicked, "It should panic on a mutation operation")

	val, _ := actual.Get(dummyKey)

	t.Equals(dummyVal, val)

	panicked, _, _ = didPanic(func() {
		actual.DeleteInverse(dummyVal)
	})
	t.Assert(panicked, "It should panic on a mutation operation")

	key, _ := actual.GetInverse(dummyVal)

	t.Equals(dummyKey, key)

	size := actual.Size()

	t.Equals(1, size)

	panicked, _, _ = didPanic(func() {
		actual.Insert("New", 1)
	})
	t.Assert(panicked, "It should panic on a mutation operation")

	size = actual.Size()

	t.Equals(1, size)

}

func TestBiMap_GetForwardMap(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()
	dummyKey := "Dummy key"
	dummyVal := 42

	forwardMap := make(map[interface{}]interface{})
	forwardMap[dummyKey] = dummyVal

	actual.Insert(dummyKey, dummyVal)

	actualForwardMap := actual.GetForwardMap()
	eq := reflect.DeepEqual(actualForwardMap, forwardMap)
	t.Assert(eq, "Forward maps should be equal")
}

func TestBiMap_GetInverseMap(tx *testing.T) {
	t := ut.BeginTest(tx, false)
	defer t.FinishTest()
	actual := NewBiMap()
	dummyKey := "Dummy key"
	dummyVal := 42

	inverseMap := make(map[interface{}]interface{})
	inverseMap[dummyVal] = dummyKey

	actual.Insert(dummyKey, dummyVal)

	actualInverseMap := actual.GetInverseMap()
	eq := reflect.DeepEqual(actualInverseMap, inverseMap)
	t.Assert(eq, "Inverse maps should be equal")
}
