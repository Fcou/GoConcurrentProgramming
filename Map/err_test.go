package Map

import (
	"fmt"
	"testing"
)

type mapKey struct {
	key int
}

func TestStruct(t *testing.T) {
	var m = make(map[mapKey]string)
	var key = mapKey{10}

	m[key] = "hello"
	fmt.Printf("m[key]=%s\n", m[key])

	// 修改key的字段的值后再次查询map，无法获取刚才add进去的值
	key.key = 100
	fmt.Printf("再次查询m[key]=%s\n", m[key])
}

func TestInit(t *testing.T) {
	var m map[int]int
	fmt.Println(m[100])
}
