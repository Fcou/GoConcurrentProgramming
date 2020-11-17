package Map

import (
	"fmt"
	"testing"

	"github.com/elliotchance/orderedmap"
)

func TestStruct(t *testing.T) {

	m := orderedmap.NewOrderedMap()

	m.Set("foo", "bar")
	m.Set("qux", 1.23)
	m.Set(123, true)

	m.Delete("qux")

	for _, key := range m.Keys() {
		value, _ := m.Get(key)
		fmt.Println(key, value)
	}
}
