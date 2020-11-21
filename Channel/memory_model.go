package main

import (
	"fmt"
	"runtime"
	"sync/atomic"
)

// var a string
// var done bool

// func setup() {
// 	a = "hello, world"
// 	done = true
// }

// func main() {
// 	go setup()
// 	for !done {
// 	}
// 	print(a)
// }

func main() {
	var a, b int32 = 0, 0

	go func() {
		atomic.StoreInt32(&a, 1)
		atomic.StoreInt32(&b, 1)
	}()

	for atomic.LoadInt32(&b) == 0 {
		runtime.Gosched() //这个函数的作用是让当前goroutine让出CPU，好让其它的goroutine获得执行的机会。同时，当前的goroutine也会在未来的某个时间点继续运行。
	}
	fmt.Println(atomic.LoadInt32(&a))
}
