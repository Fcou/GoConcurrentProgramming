package kratos

import (
	"context"
	"os"
	"os/signal"
	"syscall"
	"time"

	"golang.org/x/sync/errgroup"
)

// Lifecycle is component lifecycle.
// 抽象每个具体的服务，服务都有 start 和 stop 方法
type Lifecycle interface {
	Start(context.Context) error
	Stop(context.Context) error
}

// Hook is a pair of start and stop callbacks.
// 用Hook再包装一下 每个具体服务的抽象 start 和 stop 方法
type Hook struct {
	OnStart func(context.Context) error
	OnStop  func(context.Context) error
}

// Option is an application option.
type Option func(o *options)

// options is an application options.
type options struct {
	startTimeout time.Duration
	stopTimeout  time.Duration

	sigs  []os.Signal
	sigFn func(*App, os.Signal)
}

// StartTimeout with start timeout.
func StartTimeout(d time.Duration) Option {
	return func(o *options) { o.startTimeout = d }
}

// StopTimeout with stop timeout.
func StopTimeout(d time.Duration) Option {
	return func(o *options) { o.stopTimeout = d }
}

// Signal with os signals.
func Signal(fn func(*App, os.Signal), sigs ...os.Signal) Option {
	return func(o *options) {
		o.sigFn = fn
		o.sigs = sigs
	}
}

// App is an application components lifecycle manager
type App struct {
	opts  options
	hooks []Hook

	cancel func()
}

// New create an application lifecycle manager.
func New(opts ...Option) *App {
	options := options{
		startTimeout: time.Second * 30,
		stopTimeout:  time.Second * 30,
		sigs: []os.Signal{
			syscall.SIGTERM,
			syscall.SIGQUIT,
			syscall.SIGINT,
		},
		sigFn: func(a *App, sig os.Signal) {
			switch sig {
			case syscall.SIGINT, syscall.SIGQUIT, syscall.SIGTERM:
				a.Stop()
			default:
			}
		},
	}
	for _, o := range opts {
		o(&options)
	}
	return &App{opts: options}
}

// Append register interface that are executed on application start and stop.
func (a *App) Append(lc Lifecycle) {
	a.hooks = append(a.hooks, Hook{
		OnStart: func(ctx context.Context) error {
			return lc.Start(ctx) //将 lifecycle 抽象里的star方法（每个具体服务）包装成 Hook的方法
		},
		OnStop: func(ctx context.Context) error {
			return lc.Stop(ctx)
		},
	})
}

// AppendHook register callbacks that are executed on application start and stop.
func (a *App) AppendHook(hook Hook) {
	a.hooks = append(a.hooks, hook)
}

// Run executes all OnStart hooks registered with the application's Lifecycle.
func (a *App) Run() error {
	var ctx context.Context
	ctx, a.cancel = context.WithCancel(context.Background())
	g, ctx := errgroup.WithContext(ctx)
	for _, hook := range a.hooks {
		hook := hook //????????
		if hook.OnStop != nil {
			g.Go(func() error {
				<-ctx.Done() // wait for stop signal
				stopCtx, cancel := context.WithTimeout(context.Background(), a.opts.stopTimeout)
				defer cancel() // 如果OnStop()逻辑执行完毕，则该goroutine之后的所有逻辑代码都cancel
				return hook.OnStop(stopCtx)
			})
		}
		if hook.OnStart != nil {
			g.Go(func() error {
				startCtx, cancel := context.WithTimeout(context.Background(), a.opts.startTimeout)
				defer cancel()
				return hook.OnStart(startCtx)
			})
		}
	}
	if len(a.opts.sigs) == 0 {
		return g.Wait()
	}
	c := make(chan os.Signal, len(a.opts.sigs))
	signal.Notify(c, a.opts.sigs...)
	g.Go(func() error {
		for {
			select {
			case <-ctx.Done():
				return ctx.Err()
			case sig := <-c:
				if a.opts.sigFn != nil {
					a.opts.sigFn(a, sig)
				}
			}
		}
	})
	return g.Wait()
}

// Stop gracefully stops the application.
func (a *App) Stop() {
	if a.cancel != nil {
		a.cancel()
	}
}
