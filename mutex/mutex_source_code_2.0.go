
	/*相对于初版的设计，这次的改动主要就是，新来的 goroutine 也有机会先获取到锁，
	甚至一个 goroutine 可能连续获取到锁，打破了先来先得的逻辑。但是，代码复杂度也显而易见。*/
	type Mutex struct {
		state int32
		sema  uint32
	}

	const (
		mutexLocked      = 1 << iota // 锁持有标记  1<<0 ===1
		mutexWoken                   // 唤醒标记  1<<1 ===2
		mutexWaiterShift = iota      // 用于计算阻塞等待的waiter数量  2
	)

	func (m *Mutex) Lock() {
		// Fast path: 幸运case，能够直接获取到锁
		if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
			return
		}

		awoke := false
		for {
			old := m.state
			new := old | mutexLocked  // 新状态加锁
			if old&mutexLocked != 0 { // 锁原状态没加锁，则跳过，两位同时为“1”，结果才为“1”，否则为0
				new = old + 1<<mutexWaiterShift //锁原状态已加锁，则等待者数量加一
			}
			if awoke {
				// goroutine是被唤醒的，
				// 新状态清除唤醒标志
				new &^= mutexWoken //如果mutexWoken是0，则左侧数保持不变，如果mutexWoken是1，则左侧数一定清零
			}
			if atomic.CompareAndSwapInt32(&m.state, old, new) { //如果设置新状态成功，则代表抢夺锁的操作成功了
				if old&mutexLocked == 0 { // 锁原状态未加锁，说明是新抢到了锁，则退出循环返回。
					break
				}
				runtime.Semacquire(&m.sema) // 请求信号量，请求不到则阻塞休眠
				awoke = true
			}
		}
	}

	func (m *Mutex) Unlock() {
		// Fast path: drop lock bit.
		new := atomic.AddInt32(&m.state, -mutexLocked) //去掉锁标志
		if (new+mutexLocked)&mutexLocked == 0 {        //本来就没有加锁
			panic("sync: unlock of unlocked mutex")
		}

		old := new
		for {
			if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken) != 0 { // 没有等待者，或者这个时候有唤醒的 goroutine，或者是又被别人加了锁，那么，无需我们操劳，其它 goroutine 自己干得都很好
				return
			}
			//如果有等待者，并且没有唤醒的 waiter，那就需要唤醒一个等待的 waiter。
			new = (old - 1<<mutexWaiterShift) | mutexWoken // 将 waiter 数量减 1，并且将 mutexWoken 标志设置上，新状态，准备唤醒goroutine，并设置唤醒标志
			if atomic.CompareAndSwapInt32(&m.state, old, new) {
				runtime.Semrelease(&m.sema) //释放信号量
				return
			}
			old = m.state
		}
	}