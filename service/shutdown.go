//go:build darwin || windows || linux

package service

import (
	"context"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"sync"
	"time"
)

// Option 典型的 Option 设计模式
type Option func(*App)

// ShutdownCallback 采用 context.Context 来控制超时，而不是用 time.After 是因为
// - 超时本质上是使用这个回调的人控制的
// - 我们还希望用户知道，他的回调必须要在一定时间内处理完毕，而且他必须显式处理超时错误
type ShutdownCallback func(ctx context.Context)

// WithShutdownCallbacks 你需要实现这个方法
func WithShutdownCallbacks(cbs ...ShutdownCallback) Option {
	return func(app *App) {
		app.cbs = cbs
		/*for _, cb := range cbs {
			app.cbs = append(app.cbs, cb)
		}*/
	}
	//panic("implement me")
}

// App 这里我已经预先定义好了各种可配置字段
type App struct {
	servers []*Server

	// 优雅退出整个超时时间，默认30秒
	shutdownTimeout time.Duration

	// 优雅退出时候等待处理已有请求时间，默认10秒钟
	waitTime time.Duration
	// 自定义回调超时时间，默认三秒钟
	cbTimeout time.Duration

	cbs []ShutdownCallback
}

// NewApp 创建 App 实例，注意设置默认值，同时使用这些选项
func NewApp(servers []*Server, opts ...Option) *App {
	app := &App{
		servers:         []*Server{},
		shutdownTimeout: time.Second * 30,
		waitTime:        time.Second * 10,
		cbTimeout:       time.Second * 3,
		cbs:             []ShutdownCallback{},
	}
	for _, srv := range servers {
		app.servers = append(app.servers, srv)
	}
	for _, opt := range opts {
		opt(app)
	}
	return app
	//panic("implement me")
}

func WithShutdownTimeout(shutdownTimeout time.Duration) func(*App) {
	return func(app *App) {
		app.shutdownTimeout = shutdownTimeout
	}
}

func WithWaitTime(waitTime time.Duration) func(*App) {
	return func(app *App) {
		app.waitTime = waitTime
	}
}

func WithCBTimeout(cbTimeout time.Duration) func(*App) {
	return func(app *App) {
		app.cbTimeout = cbTimeout
	}
}

//see above WithShutdownCallbacks
/*func withCBS(cbs []ShutdownCallback) func(*App) {
	return func(app *App) {
		for _, cb := range cbs {
			append(app.cbs, cb)
		}
	}
}*/

// StartAndServe 你主要要实现这个方法
func (app *App) StartAndServe() {
	for _, s := range app.servers {
		srv := s
		go func() {
			if err := srv.Start(); err != nil {
				if err == http.ErrServerClosed {
					log.Printf("服务器%s已关闭", srv.name)
				} else {
					log.Printf("服务器%s异常退出", srv.name)
				}

			}
		}()
	}
	// 从这里开始优雅退出监听系统信号，强制退出以及超时强制退出。
	// 优雅退出的具体步骤在 shutdown 里面实现
	// 所以你需要在这里恰当的位置，调用 shutdown
	signals := []os.Signal{}
	switch runtime.GOOS {
	case "darwin":
		signals = macosSignal()
	case "linux":
		signals = linuxSignal()
	case "windows":
		signals = windowsSignal()
	}
	quit := make(chan os.Signal, 1)
	signal.Notify(quit, signals...)
	select {
	case <-quit:
		doneShutdown := make(chan struct{}, 1)
		go func() {
			//doneTimeout := make(chan time, 1)               //time replaced with any
			//doneTimeout <- time.After(app.shutdownTimeout) //should not do this, based on design requirements.？maybe needs to do this
			forceQuit := make(chan os.Signal, 1)
			signal.Notify(forceQuit, signals...)
			select {
			case <-time.After(app.shutdownTimeout):
				log.Printf("优雅退出超时，执行立即退出……")
				os.Exit(1) //which code?
			case <-forceQuit:
				log.Printf("接收到强制退出信号，执行强制退出……")
				os.Exit(2) //which code?
			case <-doneShutdown:
				log.Printf("优雅退出按时完成")
				return
			}
		}()

		app.shutdown()
		doneShutdown <- struct{}{}

		os.Exit(0)
	}

}

// shutdown 你要设计这里面的执行步骤。
func (app *App) shutdown() {
	log.Println("开始关闭应用，停止接收新请求")
	// 你需要在这里让所有的 server 拒绝新请求
	for _, s := range app.servers {
		srv := s
		srv.rejectReq()
	}

	log.Println("等待正在执行请求完结")
	// 在这里等待一段时间

	doneReq := make(chan struct{}, 1)
	skipOperation := make(chan struct{}, 1)
	wg0 := sync.WaitGroup{}
	wg0.Add(1)
	go func() {
		defer wg0.Done()
		select {
		case <-time.After(app.waitTime):
			log.Printf("执行正在等待的请求已超时")
			skipOperation <- struct{}{}
		case <-doneReq:
			log.Printf("等待中的请求现已执行完毕")
			skipOperation <- struct{}{}
		}
		return
	}()

	go func() {
		for i := 0; i < 2; i++ {
			select {
			case <-skipOperation:
				return
			default:
				//execute requests
				time.Sleep(time.Second * 3)
				doneReq <- struct{}{}
				log.Printf("goroutine执行结束")
			}
		}

	}()
	wg0.Wait()

	log.Println("开始关闭服务器")
	// 并发关闭服务器，同时要注意协调所有的 server 都关闭之后才能步入下一个阶段
	wg := sync.WaitGroup{}
	for _, s := range app.servers {
		//srv := s //why need to use srv instead of s? seems s in range is deep reference, which might change original servers?
		wg.Add(1)
		go func(srv *Server) { //pointer?
			defer wg.Done()
			if err := srv.stop(); err != nil {
				log.Printf("停止服务器 %s 出错，请检查端口 %s", srv.name, srv.srv.Addr)
				return //do we need it to wait forever here until timeout if some servers stop() return errors? if so, then we need this return here, otherwise, we can remove the return here
			}
		}(s)
	}
	wg.Wait()

	log.Println("开始执行自定义回调")
	// 并发执行回调，要注意协调所有的回调都执行完才会步入下一个阶段
	// needs to get an error so that go func returns and wait there
	//what is the logic here? if cb timeout, should we wait forever or continue to the close app? normally it should go to close app, right?
	wg1 := sync.WaitGroup{}
	for _, c := range app.cbs {
		//cb := c
		wg1.Add(1)
		go func(cb ShutdownCallback) {
			defer wg1.Done()
			ctx, cancel := context.WithTimeout(context.Background(), app.cbTimeout)
			defer cancel()
			cb(ctx)
		}(c)
	}
	wg1.Wait()

	// 释放资源
	log.Println("开始释放资源")
	app.close()
}

func (app *App) close() {
	// 在这里释放掉一些可能的资源
	time.Sleep(time.Second)
	app = nil // assign nil to pointer that points to an app object, this indicates GC to take care of the cleaning work.
	log.Println("应用关闭")
}

type Server struct {
	srv  *http.Server
	name string
	mux  *serverMux
}

// serverMux 既可以看做是装饰器模式，也可以看做委托模式
type serverMux struct {
	reject bool
	*http.ServeMux
}

// 为啥writer是值传递，request是地址传递？
func (s *serverMux) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	if s.reject {
		w.WriteHeader(http.StatusServiceUnavailable)
		_, _ = w.Write([]byte("服务已关闭"))
		log.Printf("服务器已关闭，拒绝请求")
		return
	}
	s.ServeMux.ServeHTTP(w, r)
}

// NewServer 为什么返回的是一个server的指针？
func NewServer(name string, addr string) *Server {
	mux := &serverMux{ServeMux: http.NewServeMux()}
	return &Server{
		name: name,
		mux:  mux,
		srv: &http.Server{
			Addr:    addr,
			Handler: mux,
		},
	}
}

func (s *Server) Handle(pattern string, handler http.Handler) {
	s.mux.Handle(pattern, handler)
}

func (s *Server) Start() error {
	return s.srv.ListenAndServe()
}

// 小写开头，是local函数么？并不对外暴露？
func (s *Server) rejectReq() {
	s.mux.reject = true
}

func (s *Server) stop() error {
	log.Printf("服务器%s关闭中", s.name)
	time.Sleep(time.Second * 5)
	return s.srv.Shutdown(context.Background())
}
