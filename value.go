package background

import "reflect"

type valueBackground struct {
	*group
	key   interface{}
	value interface{}
}

// WithValue returns new Background with merged children and value assigned to key.
//
// Use background values only for data that represents custom backgrounds, not
// for returning optional values from functions.
//
// Other rules for working with Value is the same as in the standard
// package context:
//
// 1. Functions that wish to store values in Background typically allocate
// a key in a global variable then use that key as the argument to
// background.WithValue and Background.Value.
//
// 2. A key can be any type that supports equality and can not be nil.
//
// 3. Packages should define keys as an unexported type to avoid
// collisions.
//
// 4. Packages that define a Background key should provide type-safe accessors
// for the values stored using that key (see examples).
func WithValue(key, value interface{}, children ...Background) Background {
	return withValue(key, value, children...)
}

func withValue(key, value interface{}, children ...Background) *valueBackground {
	if key == nil {
		panic("nil background value key")
	}

	if !reflect.TypeOf(key).Comparable() {
		panic("background value key is not comparable")
	}

	return &valueBackground{
		group: merge(children...),
		key:   key,
		value: value,
	}
}

// Value returns value assotiated with key from valueBackground or from its children,
// or nil if it is not found.
func (e *valueBackground) Value(key interface{}) (value interface{}) {
	if e.key == key {
		return e.value
	}

	return e.group.Value(key)
}

func (e *valueBackground) DependsOn(children ...Background) Background {
	return withDependency(e, children...)
}
