package get

import "sync"

var (
	_ins GormErrorPlugin
	once sync.Once
)

func NewGlobalGormErrorPlugin(opts ...Option) GormErrorPlugin {
	once.Do(func() {
		_ins = NewGormErrorPlugin(opts...)
	})
	return _ins
}
