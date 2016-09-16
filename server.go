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
	DefaultKeepAlive=60
)

var (
	abort bool
	httpAddr string
	fcgiAddr string
	fcgiSock string
	termWait int
	debug bool
	keepAlive int

)

func init() {
	flag.StringVar(&httpAddr, "http-addr", DefaultHTTPAddr, "Set the HTTP bind address")
	flag.StringVar(&fcgiAddr, "fcgi-addr", DefaultFastCGIAddr, "FastCGI Port")
	flag.StringVar(&fcgiSock, "fcgi-sock", DefaultFastSOCK, "FastCGI Socket")
	flag.IntVar(&termWait,"term-wait",0,"How long to wait between SIGTERM(docker stop) and exit")
	flag.IntVar(&keepAlive,"keep-alive", DefaultKeepAlive,"KeepAlive header timeout.  Default: 60")
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
	ln       net.Listener
	httpAddr string
	keepAlive int
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
			if s.keepAlive != 0 {
				w.Header().Set("Keep-Alive", fmt.Sprintf("timeout=%d", s.keepAlive))
			}
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
		if s.keepAlive != 0 {
			w.Header().Set("Keep-Alive", fmt.Sprintf("timeout=%d", s.keepAlive))
		}
		w.Header().Set("Content-Type", "text/plain")
		w.Header().Set("Content-Length", fmt.Sprint(len(body)))
		fmt.Fprint(w, body)

	default:
		w.WriteHeader(http.StatusMethodNotAllowed)
	}

}

// tcpKeepAliveListener sets TCP keep-alive timeouts on accepted
// connections. It's used by ListenAndServe and ListenAndServeTLS so
// dead TCP connections (e.g. closing laptop mid-download) eventually
// go away.

type tcpKeepAliveListener struct {
	*net.TCPListener
}
func (ln tcpKeepAliveListener) Accept() (c net.Conn, err error) {
	tc, err := ln.AcceptTCP()
	if err != nil {
		return
	}
	tc.SetKeepAlive(true)
	tc.SetKeepAlivePeriod(3 * time.Minute)
	return tc, nil
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

	server := Server{
		httpAddr: httpAddr,
		keepAlive: keepAlive,
	}
	var (
		tcp net.Listener
		unix net.Listener
	)
	go func() {

		httpServer := http.Server{
			Handler: server,
		}
		http.Handle("/", server)
		if server.httpAddr == "" {
			server.httpAddr = ":http"
		}
		var err error
		server.ln, err = net.Listen("tcp", server.httpAddr)
		if err != nil {
			log.Fatal(err)
		}
		logrus.Infof("Listening on %v\n",server.ln.Addr())
		if err := httpServer.Serve(tcpKeepAliveListener{server.ln.(*net.TCPListener)}); err != nil {
			logrus.Errorln("http Serve error", err)
		}
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("fcgi Server Recovered in f", r)
			}
		}()

		var err error
		tcp, err = net.Listen("tcp", fcgiAddr)
		if err != nil {
			log.Fatal(err)
		}
		fcgi.Serve(tcp, server)
	}()

	go func() {
		defer func() {
			if r := recover(); r != nil {
				fmt.Println("fcgi unix Server Recovered in f", r)
			}
		}()
		var err error
		unix, err = net.Listen("unix", fcgiSock)
		if err != nil {
			log.Fatal(err)
		}
		fcgi.Serve(unix, server)
	}()

	<-sigchan
	logrus.Infof("[%v] Got signal.  waiting %d to shutdown", time.Now(),termWait)
	tcp.Close()
	unix.Close()

	server.ln.Close()
	if termWait > 0{
		time.Sleep(time.Duration(termWait) * time.Second)
	}
	logrus.Infof("[%v] All done.  ",time.Now())
	if fcgiSock == DefaultFastSOCK {
		if _,err :=os.Stat(DefaultFastSOCK); os.IsExist(err) {
			if err := os.Remove(DefaultFastSOCK); err != nil {
				log.Fatal(err)
			}
		}
	}
}
