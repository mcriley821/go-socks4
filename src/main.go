package main

import (
	"socks4/server"

	"context"
	"errors"
	"fmt"
	"net"
	"os"
	"os/signal"
	"path"
	"time"

	"github.com/joeshaw/envdecode"
	"github.com/natefinch/lumberjack"

	"go.uber.org/zap"
	"go.uber.org/zap/zapcore"
)

type config struct {
	LogLevel   zapcore.Level `env:"LOG_LEVEL,default=info"`
	ListenIP   IP            `env:"LISTEN_IP,default=0.0.0.0"`
	ListenPort int           `env:"LISTEN_PORT,default=1080"`
}

type IP net.IP

// Decode implements the interface `envdecode.Decoder` for `IP`s
func (ip *IP) Decode(repr string) error {
	parsed := net.ParseIP(repr)
	if parsed == nil {
		return errors.New("not a valid IP")
	}
	*ip = IP(parsed)
	return nil
}

func (ip IP) String() string {
	return net.IP(ip).String()
}

func main() {
	conf := &config{}
	if err := envdecode.StrictDecode(conf); err != nil {
		println("failed to decode config from environment")
		os.Exit(1)
	}

	log := initLogging(conf)
	server := server.NewServer(log)
	addr := fmt.Sprintf("%s:%d", conf.ListenIP.String(), conf.ListenPort)

	log.Info("launching server", zap.String("listen-address", addr))
	endpoint, err := server.ListenAndServe(addr)
	if err != nil {
		log.Error("failed to launch server", zap.Error(err))
		os.Exit(1)
	}
	log.Info("listening for clients", zap.String("endpoint", endpoint.String()))

	// wait for a signal
	s := make(chan os.Signal, 1)
	signal.Notify(s, os.Interrupt)
	<-s

	log.Warn("shutting down")

	ctx, cancel := context.WithTimeout(context.Background(), time.Second*15)
	server.Close(ctx)
	cancel()
}

func initLogging(config *config) *zap.Logger {
	lvlEnable := zap.LevelEnablerFunc(func(lvl zapcore.Level) bool {
		return lvl >= config.LogLevel
	})

	filename := path.Join(os.Getenv("PREFIX"), "var", "log", "socks4", "socks4.log")

	core := zapcore.NewCore(
		zapcore.NewJSONEncoder(zap.NewProductionEncoderConfig()),
		zapcore.AddSync(&lumberjack.Logger{
			Filename:   filename,
			MaxSize:    10, // MB
			MaxBackups: 10, // Max old files
			MaxAge:     7,  // days
			Compress:   true,
		}),
		lvlEnable,
	)

	if config.LogLevel == zapcore.DebugLevel {
		debugCore := zapcore.NewCore(
			zapcore.NewConsoleEncoder(zap.NewDevelopmentEncoderConfig()),
			zapcore.Lock(os.Stdout),
			lvlEnable,
		)

		core = zapcore.NewTee(core, debugCore)
	}

	return zap.New(core)
}
