package app

import (
	"context"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/app/api"
	"github.com/hjhsamuel/agent/app/api/request"
	"github.com/hjhsamuel/agent/config"
	"github.com/hjhsamuel/agent/internal/service"
)

func Start() error {
	conf, err := config.Init()
	if err != nil {
		return err
	}

	srv, err := service.NewService(conf)
	if err != nil {
		return err
	}
	if err = srv.Start(); err != nil {
		return err
	}
	defer srv.Close()

	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(request.AuthMiddleware)
	request.SetTokenSalt(conf.Server.Salt)
	api.Register(srv, r)

	httpSrv := &http.Server{
		Addr:    net.JoinHostPort(conf.Server.Host, strconv.Itoa(conf.Server.Port)),
		Handler: r,
	}
	go func() {
		_ = httpSrv.ListenAndServe()
	}()

	sig := make(chan os.Signal, 1)
	signal.Notify(sig, syscall.SIGINT, syscall.SIGTERM)
	<-sig

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	_ = httpSrv.Shutdown(ctx)

	return nil
}
