// Copyright 2018 Alexander Zaytsev <thebestzorro@yandex.ru>.
// All rights reserved. Use of this source code is governed
// by a MIT-style license that can be found in the LICENSE file.

// Package conf implements methods setup configuration settings.
package main

import (
	"context"
	"flag"
	"fmt"
	"log"
	"net/http"
	"os"
	"os/signal"
	"runtime"
	"syscall"
	"time"

	"github.com/z0rr0/enigma/conf"
	"github.com/z0rr0/enigma/db"
	"github.com/z0rr0/enigma/web"
)

const (
	// Name is a program name
	Name = "Enigma"
	// Config is default configuration file name
	Config = "config.json"
)

var (
	// Version is git version
	Version = ""
	// Revision is revision number
	Revision = ""
	// BuildDate is build date
	BuildDate = ""
	// GoVersion is runtime Go language version
	GoVersion = runtime.Version()

	// internal loggers
	loggerError = log.New(os.Stderr, fmt.Sprintf("ERROR [%v]: ", Name),
		log.Ldate|log.Ltime|log.Lshortfile)
	loggerInfo = log.New(os.Stdout, fmt.Sprintf("INFO [%v]: ", Name),
		log.Ldate|log.Ltime|log.Lshortfile)
)

func getVersion(w http.ResponseWriter, cfg *conf.Cfg) error {
	conn := cfg.Connection()
	defer func() {
		err := conn.Close()
		if err != nil {
			loggerError.Printf("failed connection close: %v\n", err)
		}
	}()
	_, err := fmt.Fprintf(w,
		"%v\nVersion: %v\nRevision: %v\nBuild date: %v\nGo version: %v\nDb is OK: %v\n",
		Name, Version, Revision, BuildDate, GoVersion, db.IsOk(conn),
	)
	return err
}

func main() {
	defer func() {
		if r := recover(); r != nil {
			loggerError.Printf("abnormal termination [%v]: \n\t%v\n", Version, r)
		}
	}()
	version := flag.Bool("version", false, "show version")
	config := flag.String("config", Config, "configuration file")
	flag.Parse()

	versionInfo := fmt.Sprintf("\tVersion: %v\n\tRevision: %v\n\tBuild date: %v\n\tGo version: %v",
		Version, Revision, BuildDate, GoVersion)
	if *version {
		fmt.Println(versionInfo)
		return
	}

	cfg, err := conf.New(*config)
	if err != nil {
		panic(err)
	}
	defer func() {
		err := cfg.Close()
		if err != nil {
			loggerError.Println("failed connection close after stop")
		}
	}()
	timeout := cfg.HandleTimeout()
	srv := &http.Server{
		Addr:           cfg.Addr(),
		Handler:        http.DefaultServeMux,
		ReadTimeout:    timeout,
		WriteTimeout:   timeout,
		MaxHeaderBytes: 1 << 20, // 1MB
		ErrorLog:       loggerInfo,
	}
	loggerInfo.Printf("\n%v\nlisten addr: %v\n", versionInfo, srv.Addr)
	http.HandleFunc("/", func(w http.ResponseWriter, r *http.Request) {
		var err error
		start, code := time.Now(), http.StatusOK
		defer func() {
			loggerInfo.Printf("%-5v %v\t%-12v\t%v",
				r.Method,
				code,
				time.Since(start),
				r.URL.String(),
			)
		}()
		switch r.URL.Path {
		case "/version":
			code, err = http.StatusOK, getVersion(w, cfg)
		case "/":
			code, err = web.Index(w, r, cfg)
		default:
			code, err = web.Read(w, r, cfg)
		}
		if err != nil {
			loggerError.Println(err)
		}
	})

	idleConnsClosed := make(chan struct{})
	go func() {
		sigint := make(chan os.Signal, 1)
		signal.Notify(sigint, os.Interrupt, os.Signal(syscall.SIGTERM))
		<-sigint

		if err := srv.Shutdown(context.Background()); err != nil {
			loggerInfo.Printf("HTTP server Shutdown: %v", err)
		}
		close(idleConnsClosed)
	}()

	if err := srv.ListenAndServe(); err != http.ErrServerClosed {
		loggerInfo.Printf("HTTP server ListenAndServe: %v", err)
	}
	<-idleConnsClosed
	loggerInfo.Println("stopped")
}
