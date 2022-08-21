/*
package gracefulShutDown

import (

	"context"
	"gracefulShutDown/service"
	"log"
	"net/http"
	"time"

)
*/
package main

import (
	"context"
	"gracefulShutDown/service"
	"log"
	"net/http"
	"time"
)

// 注意要从命令行启动，否则不同的 IDE 可能会吞掉关闭信号
func main() {
	//fmt.Println("Enter shutdown timeout value:\n")
	//timeConfig := make(chan int32, 1)
	//inputTimeoutValue := int32(60)
	//inputTimeoutValue <- timeConfig
	//shutdownTimeout := time.Second * inputTimeoutValue
	s1 := service.NewServer("business", "localhost:8080")
	s1.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("hello, business"))
	})) //why here is a http.HandlerFunc without return?
	s2 := service.NewServer("admin", "localhost:8081")
	s2.Handle("/", http.HandlerFunc(func(writer http.ResponseWriter, request *http.Request) {
		_, _ = writer.Write([]byte("hello, admin"))
	}))
	//app := service.NewApp([]*service.Server{s1, s2}, service.WithShutdownCallbacks(StoreCacheToDBCallback))
	app := service.NewApp([]*service.Server{s1, s2},
		service.WithShutdownTimeout(time.Second*20),
		service.WithWaitTime(time.Second*2),
		service.WithCBTimeout(time.Second*3),
		service.WithShutdownCallbacks(StoreCacheToDBCallback),
	)
	app.StartAndServe()
}

func StoreCacheToDBCallback(ctx context.Context) {
	done := make(chan struct{}, 1)
	go func() {
		// 你的业务逻辑，比如说这里我们模拟的是将本地缓存刷新到数据库里面
		// 这里我们简单的睡一段时间来模拟
		log.Printf("刷新缓存中……")
		time.Sleep(100 * time.Millisecond)
		log.Printf("本次运行log记录中……")
		time.Sleep(200 * time.Millisecond)
		log.Printf("新数据加密中……")
		time.Sleep(400 * time.Millisecond)
		log.Printf("新数据写入数据库中……")
		time.Sleep(500 * time.Millisecond)
		log.Printf("缓存写入数据库已完成，正在删除缓存……")
		time.Sleep(1 * time.Second)
		//log.Printf("数据库刷新已完成")
		done <- struct{}{} //close(done)
	}()

	//not sure if I should do it like this?
	//go func() {
	//	ctx.withTimeout(ctx, cbTimeout)
	//}()

	select {
	case <-ctx.Done():
		log.Printf("刷新缓存超时")
	case <-done:
		log.Printf("缓存被刷新到了 DB")
	}
}
