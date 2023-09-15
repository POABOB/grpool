package grpool

import "time"

// 參數設定
type Option func(opts *Options)

// 加載設定
func loadOptions(options ...Option) *Options {
	opts := new(Options)
	for _, option := range options {
		option(opts)
	}
	return opts
}

// 該設定會被用來引入至 Pool 中
type Options struct {
	// 過期時間: 用於定時清理過期的 Worker (只要太久沒被使用的 Worker 就會被清理)，預設為 1 秒
	ExpiryDuration time.Duration

	// 是否提前申請空間，大量執行需求中使用
	PreAlloc bool

	// Nonblocking 用來阻塞任務
	// 若設定為 true，就會返回 ErrPoolOverload 錯誤
	Nonblocking bool

	// 用來處理 worker panic 發生的事件
	PanicHandler func(interface{})

	// 若設定為 true，Worker 就不會被自動清除
	DisableClear bool
}

// 直接傳入 Options
func WithOptions(options Options) Option {
	return func(opts *Options) {
		*opts = options
	}
}

// 設定過期時間
func WithExpiryDuration(expiryDuration time.Duration) Option {
	return func(opts *Options) {
		opts.ExpiryDuration = expiryDuration
	}
}

// 設定是否要提前創建空間
func WithPreAlloc(preAlloc bool) Option {
	return func(opts *Options) {
		opts.PreAlloc = preAlloc
	}
}

// 若為 true，代表沒有有用的 Worker 時，會直接 ErrPoolOverload
func WithNonblocking(nonblocking bool) Option {
	return func(opts *Options) {
		opts.Nonblocking = nonblocking
	}
}

// Panic 事件處理
func WithPanicHandler(panicHandler func(interface{})) Option {
	return func(opts *Options) {
		opts.PanicHandler = panicHandler
	}
}

// 是否要關閉 Clear
func WithDisableClear(disable bool) Option {
	return func(opts *Options) {
		opts.DisableClear = disable
	}
}
