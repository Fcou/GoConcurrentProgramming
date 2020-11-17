package Pool

import "sync/atomic"

func poolCleanup() {
	// 丢弃当前victim, STW所以不用加锁
	for _, p := range oldPools {
		p.victim = nil
		p.victimSize = 0
	}

	// 将local复制给victim, 并将原local置为nil
	for _, p := range allPools {
		p.victim = p.local
		p.victimSize = p.localSize
		p.local = nil
		p.localSize = 0
	}

	oldPools, allPools = allPools, nil
}

func (p *Pool) Get() interface{} {
	// 把当前goroutine固定在当前的P上
	l, pid := p.pin()
	x := l.private // 优先从local的private字段取，快速
	l.private = nil
	if x == nil {
		// 从当前的local.shared弹出一个，注意是从head读取并移除
		x, _ = l.shared.popHead()
		if x == nil { // 如果没有，则去偷一个
			x = p.getSlow(pid)
		}
	}
	runtime_procUnpin()
	// 如果没有获取到，尝试使用New函数生成一个新的
	if x == nil && p.New != nil {
		x = p.New()
	}
	return x
}

func (p *Pool) getSlow(pid int) interface{} {

	size := atomic.LoadUintptr(&p.localSize)
	locals := p.local
	// 从其它proc中尝试偷取一个元素
	for i := 0; i < int(size); i++ {
		l := indexLocal(locals, (pid+i+1)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// 如果其它proc也没有可用元素，那么尝试从vintim中获取
	size = atomic.LoadUintptr(&p.victimSize)
	if uintptr(pid) >= size {
		return nil
	}
	locals = p.victim
	l := indexLocal(locals, pid)
	if x := l.private; x != nil { // 同样的逻辑，先从vintim中的local private获取
		l.private = nil
		return x
	}
	for i := 0; i < int(size); i++ { // 从vintim其它proc尝试偷取
		l := indexLocal(locals, (pid+i)%int(size))
		if x, _ := l.shared.popTail(); x != nil {
			return x
		}
	}

	// 如果victim中都没有，则把这个victim标记为空，以后的查找可以快速跳过了
	atomic.StoreUintptr(&p.victimSize, 0)

	return nil
}

func (p *Pool) Put(x interface{}) {
	if x == nil { // nil值直接丢弃
		return
	}
	l, _ := p.pin()
	if l.private == nil { // 如果本地private没有值，直接设置这个值即可
		l.private = x
		x = nil
	}
	if x != nil { // 否则加入到本地队列中
		l.shared.pushHead(x)
	}
	runtime_procUnpin()
}
