
	/*在 2015 年 2 月的改动中，如果新来的 goroutine 或者是被唤醒的 goroutine 首次获取不到锁，它们就会通过
	自旋（spin，通过循环不断尝试，spin 的逻辑是在runtime 实现的）的方式，尝试检查锁是否被释放。在尝试一定的自旋次数后，再执行原来的逻辑。*/
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
		// Fast path: 幸运之路，正好获取到锁
		if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
			return
		}

		awoke := false
		iter := 0 //限制自旋次数
		for {     // 不管是新来的请求锁的goroutine, 还是被唤醒的goroutine，都不断尝试请求锁
			old := m.state            // 先保存当前锁的状态
			new := old | mutexLocked  // 新状态设置加锁标志
			if old&mutexLocked != 0 { // 锁还没被释放
				if runtime_canSpin(iter) { // 还可以自旋
					if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
						atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
						awoke = true
					}
					runtime_doSpin() //自旋，如果可以 spin 的话，对于临界区代码执行非常短的场景来说，这是一个非常好的优化。
					//因为临界区的代码耗时很短，锁很快就能释放，而抢夺锁的 goroutine 不用通过休眠唤醒方式等待调度，直接 spin 几次，可能就获得了锁。
					iter++
					continue // 自旋，返回开头，再次尝试请求锁
				}
				new = old + 1<<mutexWaiterShift
			}
			if awoke { // 唤醒状态
				if new&mutexWoken == 0 {
					panic("sync: inconsistent mutex state")
				}
				new &^= mutexWoken // 新状态清除唤醒标记
			}
			if atomic.CompareAndSwapInt32(&m.state, old, new) {
				if old&mutexLocked == 0 { // 旧状态锁已释放，新状态成功持有了锁，直接返回
					break
				}
				runtime_Semacquire(&m.sema) // 阻塞等待
				awoke = true                // 被唤醒
				iter = 0
			}
		}
	}

	