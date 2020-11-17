package Map

import (
	"sync/atomic"
	"unsafe"
)

type Map struct {
	mu Mutex
	// 基本上你可以把它看成一个并发安全的只读的map， atomic.Value保证并发安全，只读靠代码逻辑保证
	// 它包含的元素其实也是通过原子操作更新的，但是已删除的entry就需要加锁操作了
	read atomic.Value // readOnly

	// 包含需要加锁才能访问的元素
	// 包括所有在read字段中但未被expunged（删除）的元素以及新加的元素
	dirty map[interface{}]*entry

	// 记录从read中读取miss的次数，一旦miss数和dirty长度一样了，就会把dirty提升为read，并把dirty置空
	misses int
}

type readOnly struct {
	m       map[interface{}]*entry
	amended bool // 当dirty中包含read没有的数据时为true，比如新增一条数据
}

// expunged是用来标识此项已经删掉的指针
// 当map中的一个项目被删除了，只是把它的值标记为expunged，以后才有机会真正删除此项
var expunged = unsafe.Pointer(new(interface{}))

// entry代表一个值
type entry struct {
	p unsafe.Pointer //  指向任意类型的指针 *interface{}
}

func (m *Map) Store(key, value interface{}) {
	read, _ := m.read.Load().(readOnly)
	// 如果read字段包含这个项，说明是更新，cas更新项目的值即可
	if e, ok := read.m[key]; ok && e.tryStore(&value) {
		return
	}

	// read中不存在，或者cas更新失败，就需要加锁访问dirty了
	m.mu.Lock()
	read, _ = m.read.Load().(readOnly)
	if e, ok := read.m[key]; ok { // 双检查，看看read是否已经存在了
		if e.unexpungeLocked() {
			// 此项目先前已经被删除了，通过将它的值设置为nil，标记为unexpunged
			m.dirty[key] = e
		}
		e.storeLocked(&value) // 更新
	} else if e, ok := m.dirty[key]; ok { // 如果dirty中有此项
		e.storeLocked(&value) // 直接更新
	} else { // 否则就是一个新的key
		if !read.amended { //如果dirty为nil
			// 需要创建dirty对象，并且标记read的amended为true,
			// 说明有元素它不包含而dirty包含
			m.dirtyLocked()
			m.read.Store(readOnly{m: read.m, amended: true})
		}
		m.dirty[key] = newEntry(value) //将新值增加到dirty对象中
	}
	m.mu.Unlock()
}

func (m *Map) dirtyLocked() {
	if m.dirty != nil { // 如果dirty字段已经存在，不需要创建了
		return
	}

	read, _ := m.read.Load().(readOnly) // 获取read字段
	m.dirty = make(map[interface{}]*entry, len(read.m))
	for k, e := range read.m { // 遍历read字段
		if !e.tryExpungeLocked() { // 把非punged的键值对复制到dirty中
			m.dirty[k] = e
		}
	}
}

func (m *Map) Load(key interface{}) (value interface{}, ok bool) {
	// 因read只读，线程安全，优先读取
	read, _ := m.read.Load().(readOnly)
	e, ok := read.m[key]
	// 如果read没有，并且dirty有新数据，那么去dirty中查找
	if !ok && read.amended { // 如果不存在并且dirty不为nil(有新的元素)
		m.mu.Lock()
		// 双重检查（原因是前文的if判断和加锁非原子的，害怕这中间发生故事），看看read中现在是否存在此key
		read, _ = m.read.Load().(readOnly)
		e, ok = read.m[key]
		if !ok && read.amended { //依然不存在，并且dirty不为nil
			e, ok = m.dirty[key] // 从dirty中读取
			// 不管dirty中存不存在，miss数都加1
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if !ok {
		return nil, false
	}
	return e.load() //返回读取的对象，e既可能是从read中获得的，也可能是从dirty中获得的
}

func (m *Map) missLocked() {
	m.misses++                   // misses计数加一
	if m.misses < len(m.dirty) { // 如果没达到阈值(dirty字段的长度),返回
		return
	}
	// 因为写操作只会操作dirty，所以保证了dirty是最新的，并且数据集是肯定包含read的。
	m.read.Store(readOnly{m: m.dirty}) //把dirty字段的内存提升为read字段
	m.dirty = nil                      // 清空dirty
	m.misses = 0                       // misses数重置为0
}

func (m *Map) Delete(key interface{}) {
	m.LoadAndDelete(key)
}
func (m *Map) LoadAndDelete(key interface{}) (value interface{}, loaded bool) {
	read, _ := m.read.Load().(readOnly)
	e, ok := read.m[key]
	// 如果read中没有，并且dirty中有新元素，那么就去dirty中去找。
	if !ok && read.amended {
		m.mu.Lock()
		// 双检查
		read, _ = m.read.Load().(readOnly)
		e, ok = read.m[key]
		if !ok && read.amended {
			e, ok = m.dirty[key]
			// 这一行长坤在1.15中实现的时候忘记加上了，导致在特殊的场景下有些key总是没有被回收
			delete(m.dirty, key) //dirty是直接删除
			// miss数加1
			m.missLocked()
		}
		m.mu.Unlock()
	}
	if ok {
		return e.delete() // 如果read中存在该key，则将该value 赋值nil（采用标记的方式删除！）
	}
	return nil, false
}

func (e *entry) delete() (value interface{}, ok bool) {
	for {
		p := atomic.LoadPointer(&e.p)
		if p == nil || p == expunged {
			return nil, false
		}
		//如果项目不为 nil 或者没有被标记为 expunged，那么还可以把它的值返回。
		if atomic.CompareAndSwapPointer(&e.p, p, nil) {
			return *(*interface{})(p), true
		}
	}
}
