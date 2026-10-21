package app

import (
	"context"
	"errors"
	"fmt"
	"net"
	"net/http"
	"os"
	"os/signal"
	"strconv"
	"syscall"
	"time"

	"github.com/gin-gonic/gin"
	"github.com/hjhsamuel/agent/api"
	"github.com/hjhsamuel/agent/api/common"
	"github.com/hjhsamuel/agent/api/common/middleware"
	"github.com/hjhsamuel/agent/config"
	"github.com/hjhsamuel/agent/internal/service"
	"github.com/sirupsen/logrus"
)

func Start() error {
	if err := config.Init(); err != nil {
		return err
	}

	srv, err := startServer()
	if err != nil {
		return err
	}
	defer srv.Close()

	httpSrv, err := startWeb(srv)
	if err != nil {
		return err
	}

	exit := make(chan os.Signal, 1)
	signal.Notify(exit, syscall.SIGINT, syscall.SIGTERM)
	<-exit
	logrus.Infof("Shutting down server...")

	ctx, cancel := context.WithTimeout(context.Background(), 5*time.Second)
	defer cancel()
	if err = httpSrv.Shutdown(ctx); err != nil {
		logrus.Errorf("Server forced to shutdown: %v", err)
	}

	return nil
}

func startServer() (*service.Service, error) {
	srv, err := service.NewService()
	if err != nil {
		return nil, err
	}
	if err = srv.Start(); err != nil {
		return nil, err
	}

	return srv, nil
}

func startWeb(srv *service.Service) (*http.Server, error) {
	gin.SetMode(gin.ReleaseMode)
	r := gin.New()
	r.Use(gin.Recovery())
	r.Use(middleware.RequestLogger)
	api.Route(srv, r)

	c := config.GetConfig()

	common.SetTokenKey(c.Server.Header)
	common.SetTokenSalt(c.Server.Salt)

	httpSrv := http.Server{
		Addr:    net.JoinHostPort(c.Server.Host, strconv.Itoa(c.Server.Port)),
		Handler: r,
	}

	listener, err := net.Listen("tcp", httpSrv.Addr)
	if err != nil {
		return nil, fmt.Errorf("listen HTTP server on %s: %w", httpSrv.Addr, err)
	}

	go func() {
		if err := httpSrv.Serve(listener); err != nil && !errors.Is(err, http.ErrServerClosed) {
			logrus.WithError(err).Error("HTTP server stopped unexpectedly")
		}
	}()

	logrus.Infof("Server started, listening at: %s", listener.Addr())

	return &httpSrv, nil
}
