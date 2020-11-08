// package main

// import "fmt"

// const (
// 	mutexLocked      = 1 << iota // 锁持有标记  1<<0 ===1
// 	mutexWoken                   // 唤醒标记  1<<1 ===2
// 	mutexWaiterShift = iota      // 用于计算阻塞等待的waiter数量  0
// )

// func main() {
// 	fmt.Printf("%d,%d,%d\n", mutexLocked, mutexWoken, mutexWaiterShift)
// }
