package Once

import "sync/atomic"

type Once struct {
	done uint32
}

func (o *Once) Do(f func()) {
	if !atomic.CompareAndSwapUint32(&o.done, 0, 1) {
		return
	}
	f() //并发下，不保证会执行完
}

//以下为正确版本
type Once struct {
	done uint32
	m    Mutex
}

func (o *Once) Do(f func()) {
	if atomic.LoadUint32(&o.done) == 0 {
		o.doSlow(f) //Go不会内联包含循环的方法。实际上，包含以下内容的方法都不会被内联：闭包调用，select，for，defer，go关键字创建的协程。并且除了这些，还有其它的限制。当解析AST时，Go申请了80个节点作为内联的预算。每个节点都会消耗一个预算。比如，a = a + 1这行代码包含了5个节点：AS, NAME, ADD, NAME, LITERAL。以下是对应的SSA dump：
	}
}

//即使此时有多个 goroutine 同时进入了 doSlow 方法，因为双检查的机制，后续的 goroutine 会看到 o.done 的值为 1，也不会再次执行 f。
func (o *Once) doSlow(f func()) {
	o.m.Lock()
	defer o.m.Unlock()
	// 双检查
	if o.done == 0 {
		defer atomic.StoreUint32(&o.done, 1)
		f()
	}
}
