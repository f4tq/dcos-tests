package main

import (
	"fmt"
	"log"
	"net"
	"net/http"
	"net/http/fcgi"
	"os"
	"os/signal"
	"syscall"
	"github.com/Sirupsen/logrus"
	"strings"
	"strconv"
	"flag"
	"time"
)

const (
	DefaultHTTPAddr = ":8080"
	DefaultFastCGIAddr = ":9001"
	DefaultFastSOCK = "/tmp/go.sock"

)

var (
	abort bool
	httpAddr string
	fcgiAddr string
	fcgiSock string
	termWait int
	debug bool
)

func init() {
	flag.StringVar(&httpAddr, "http-addr", DefaultHTTPAddr, "Set the HTTP bind address")
	flag.StringVar(&fcgiAddr, "fcgi-addr", DefaultFastCGIAddr, "FastCGI Port")
	flag.StringVar(&fcgiSock, "fcgi-sock", DefaultFastSOCK, "FastCGI Socket")
	flag.IntVar(&termWait,"term-wait",0,"How long to wait between SIGTERM(docker stop) and exit")
	flag.BoolVar(&debug,"debug",false,"Turn on debug level logging")
	flag.Usage = func() {
		logrus.Errorf("Usage: %s [options]  \n", os.Args[0])
		logrus.Errorf(` Simulates latent clients and clients that need\n
		A lot of time to shutdown.  For use with drain testing in a mesos environment\n
		Use URI /sleep/60 to sleep for 60 second\n`)
		flag.PrintDefaults()
	}
}


type Server struct {


}

func (s Server) ServeHTTP(w http.ResponseWriter, r *http.Request) {
	logrus.Debugf("logrus ServeHTTP.  '%s'  method: %s", r.URL.Path,r.Method)
	if strings.HasPrefix(r.URL.Path, "/sleep") && r.Method == "GET" {
		// allow skopos requests through without direction
		periodFunc := func() int {
			parts := strings.Split(r.URL.Path, "/")
			logrus.Debugf("/sleep parts %v len(parts)=%d",parts,len(parts))
			if len(parts) != 3 {

				return 0
			}
			if ii,err := strconv.Atoi(parts[2]); err != nil {
				logrus.Error(err)
				return 0
			} else {
				return ii
			}
		}
		period:=periodFunc()
		if period > 0 {
			now := time.Now()
			time.Sleep(time.Duration(period) * time.Second )
			later := time.Now()
			body := fmt.Sprintf("[%v] slept for %d seconds starting @%s", later.String(), period, now.String())
			w.Header().Set("Server", "gophr")
			w.Header().Set("Connection", "keep-alive")
			w.Header().Set("Content-Type", "text/plain")
			w.Header().Set("Content-Length", fmt.Sprint(len(body)))
			fmt.Fprint(w, body)
			return
		}
	}
	switch r.Method {
	case "GET":
		now := time.Now()
		body := fmt.Sprintf("[%v] Hello World\n",now.String())
		// Try to keep the same amount of headers
		w.Header().Set("Server", "gophr")
		w.Header().Set("Connection", "keep-alive")
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		fmt.Fprint(w, body)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

func main() {
	logrus.SetOutput(os.Stderr)

	if len(os.Args) == 0{
		flag.Usage()
		os.Exit(-1)
	}
	flag.Parse()
	if len(flag.Args()) != 0{
		flag.Usage()
		os.Exit(-1)
	}
	if debug {
		logrus.SetLevel(logrus.DebugLevel)
	}

	sigchan := make(chan os.Signal, 1)
	signal.Notify(sigchan, os.Interrupt)
	signal.Notify(sigchan, syscall.SIGTERM)

	server := Server{}

	go func() {
		http.Handle("/", server)

		if err := http.ListenAndServe(httpAddr, nil); err != nil {
			log.Fatal(err)
		}
	}()

	go func() {
		tcp, err := net.Listen("tcp", fcgiAddr)
		if err != nil {
			log.Fatal(err)
		}
		fcgi.Serve(tcp, server)
	}()

	go func() {
		unix, err := net.Listen("unix", fcgiSock)
		if err != nil {
			log.Fatal(err)
		}
		fcgi.Serve(unix, server)
	}()

	<-sigchan
	logrus.Debugf("[%v] Got signal.  waiting %d to shutdown", time.Now(),termWait)
	if termWait > 0{
		time.Sleep(time.Duration(termWait) * time.Second)
	}
	logrus.Debugf("[%v] All done.  ",time.Now())
	if fcgiSock == DefaultFastSOCK {
		if err := os.Remove(DefaultFastSOCK); err != nil {
			log.Fatal(err)
		}
	}
}
