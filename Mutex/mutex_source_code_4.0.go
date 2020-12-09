/*
正常模式下，waiter 都是进入先入先出队列，被唤醒的 waiter 并不会直接持有锁，而是要和新来的 goroutine 进行竞争。
新来的 goroutine 有先天的优势，它们正在 CPU 中运行，可能它们的数量还不少，所以，在高并发情况下，被唤醒的 waiter 可能比较悲剧地获取不到锁，
这时，它会被插入到队列的前面。如果 waiter 获取不到锁的时间超过阈值 1 毫秒，那么，这个 Mutex 就进入到了饥饿模式。

在饥饿模式下，Mutex 的拥有者将直接把锁交给队列最前面的 waiter。新来的 goroutine 不会尝试获取锁，
即使看起来锁没有被持有，它也不会去抢，也不会 spin，它会乖乖地加入到等待队列的尾部。

如果拥有 Mutex 的 waiter 发现下面两种情况的其中之一，它就会把这个 Mutex 转换成正常模式:
此 waiter 已经是队列中的最后一个 waiter 了，没有其它的等待锁的 goroutine 了；
此 waiter 的等待时间小于 1 毫秒
*/
type Mutex struct {
	state int32
	sema  uint32
}

const (
	mutexLocked      = 1 << iota // mutex is locked  ,1
	mutexWoken                   //2
	mutexStarving                // 从state字段中分出一个饥饿标记 //4
	mutexWaiterShift = iota      //3

	starvationThresholdNs = 1e6
)

func (m *Mutex) Lock() {
	// Fast path: 幸运之路，一下就获取到了锁
	if atomic.CompareAndSwapInt32(&m.state, 0, mutexLocked) {
		return
	}
	// Slow path：缓慢之路，尝试自旋竞争或饥饿状态下饥饿goroutine竞争
	m.lockSlow()
}

func (m *Mutex) lockSlow() {
	var waitStartTime int64 // 记录此 goroutine 请求锁的初始时间
	starving := false       // 此goroutine的饥饿标记
	awoke := false          // 唤醒标记
	iter := 0               // 自旋次数
	old := m.state          // 当前的锁的状态
	for {
		// 锁是非饥饿状态，锁还没被释放，尝试自旋
		if old&(mutexLocked|mutexStarving) == mutexLocked && runtime_canSpin(iter) {
			if !awoke && old&mutexWoken == 0 && old>>mutexWaiterShift != 0 &&
				atomic.CompareAndSwapInt32(&m.state, old, old|mutexWoken) {
				awoke = true
			}
			runtime_doSpin()
			iter++
			old = m.state // 再次获取锁的状态，之后会检查是否锁被释放了
			continue
		}

		new := old
		if old&mutexStarving == 0 {
			new |= mutexLocked // 非饥饿状态，加锁
		}
		if old&(mutexLocked|mutexStarving) != 0 {
			new += 1 << mutexWaiterShift // waiter数量加1
		}
		if starving && old&mutexLocked != 0 {
			new |= mutexStarving // 设置饥饿状态
		}
		if awoke {
			if new&mutexWoken == 0 {
				throw("sync: inconsistent mutex state")
			}
			new &^= mutexWoken // 新状态清除唤醒标记
		}
		// 成功设置新状态
		if atomic.CompareAndSwapInt32(&m.state, old, new) {
			// 原来锁的状态已释放，并且不是饥饿状态，正常请求到了锁，返回
			if old&(mutexLocked|mutexStarving) == 0 {
				break // locked the mutex with CAS
			}
			// 处理饥饿状态

			// 如果以前就在队列里面，加入到队列头，判断是否第一次加入到 waiter 队列
			queueLifo := waitStartTime != 0
			if waitStartTime == 0 {
				waitStartTime = runtime_nanotime()
			}
			// 阻塞等待
			runtime_SemacquireMutex(&m.sema, queueLifo, 1)
			// 唤醒之后检查锁是否应该处于饥饿状态
			starving = starving || runtime_nanotime()-waitStartTime > starvationThresholdNs
			old = m.state
			// 如果锁已经处于饥饿状态，直接抢到锁，返回
			if old&mutexStarving != 0 {
				if old&(mutexLocked|mutexWoken) != 0 || old>>mutexWaiterShift == 0 {
					throw("sync: inconsistent mutex state")
				}
				// 有点绕，加锁并且将waiter数减1
				delta := int32(mutexLocked - 1<<mutexWaiterShift)
				if !starving || old>>mutexWaiterShift == 1 {
					delta -= mutexStarving // 最后一个waiter或者已经不饥饿了，清除饥饿标记
				}
				atomic.AddInt32(&m.state, delta)
				break
			}
			awoke = true
			iter = 0
		} else {
			old = m.state
		}
	}
}

func (m *Mutex) Unlock() {
	// Fast path: drop lock bit.
	new := atomic.AddInt32(&m.state, -mutexLocked)
	if new != 0 {
		m.unlockSlow(new)
	}
}

func (m *Mutex) unlockSlow(new int32) {
	if (new+mutexLocked)&mutexLocked == 0 {
		throw("sync: unlock of unlocked mutex")
	}
	if new&mutexStarving == 0 {
		old := new
		for {
			if old>>mutexWaiterShift == 0 || old&(mutexLocked|mutexWoken|mutexStarving) != 0 {
				return
			}
			new = (old - 1<<mutexWaiterShift) | mutexWoken
			if atomic.CompareAndSwapInt32(&m.state, old, new) {
				runtime_Semrelease(&m.sema, false, 1)
				return
			}
			old = m.state
		}
	} else {
		runtime_Semrelease(&m.sema, true, 1)
	}
}