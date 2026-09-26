package db

// ErrorKind 是跨库错误分类。值就是分类名，不使用序号。
// 各驱动的错误码留在方言包，会话只返回这一类。
// 外键、非空、语法和断线都是 ErrorOther，避免和可重试错误混在一起。
// 零值是空串，不是 ErrorOther。未归类必须返回 ErrorOther。
type ErrorKind string

const (
	// ErrorOther 表示未归类。换库重试逻辑不要把这类当成可重试。
	ErrorOther ErrorKind = "other"
	// ErrorUnique 是主键或唯一约束冲突。同一条数据再执行一次仍会失败。
	ErrorUnique ErrorKind = "unique"
	// ErrorDeadlock 是死锁。应回滚后重试整段事务。
	ErrorDeadlock ErrorKind = "deadlock"
	// ErrorLock 是锁等待超时或数据库忙。应重试整段事务。
	ErrorLock ErrorKind = "lock"
	// ErrorSerialization 是隔离级别下的序列化失败。应重试整段事务。
	ErrorSerialization ErrorKind = "serialization"
)

// Retryable 报告整段事务重试是否有意义。唯一冲突返回 false。
func (k ErrorKind) Retryable() bool {
	switch k {
	case ErrorDeadlock, ErrorLock, ErrorSerialization:
		return true
	default:
		return false
	}
}

// String 返回分类名。未知值按未归类处理。
func (k ErrorKind) String() string {
	switch k {
	case ErrorOther, ErrorUnique, ErrorDeadlock, ErrorLock, ErrorSerialization:
		return string(k)
	default:
		return string(ErrorOther)
	}
}

// Classify 按当前方言识别 err。nil 和未实现 ErrorDialect 的引擎返回 ErrorOther。
// ClickHouse 实现了该接口并始终返回 ErrorOther：重复键通常不报错。
func (d *DB) Classify(err error) ErrorKind {
	if err == nil || d == nil || d.dial == nil {
		return ErrorOther
	}
	c, ok := d.dial.(ErrorDialect)
	if !ok {
		return ErrorOther
	}
	return c.Classify(err)
}
