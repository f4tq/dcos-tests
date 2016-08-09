Mesos slave drain tests
=======================
Server opposing locust.  This project is used in a mesos drain test.  The idea is that some containers need time to drain after mesos-slave has been stopped.  Use `--term-wait` to pause between SIGTERM and exit.

# Quickstart
```
go get github.com/f4tq/dcos-tests
```
- or-
```
docker pull docker.io/f4tq/dcos-tests
```

# Build
- clone
```
git clone https://github.com/f4tq/dcos-tests.git
```
- build with docker
Builds a docker dev image for this project.
```
make dev
```
- without docker
```
make install-deps
go build

```

## Requirements
- golang 1.6
- govendor
- make
- or docker

# Run
```
dcos-tests
Usage:

  -debug
        Turn on debug level logging
  -fcgi-addr string
        FastCGI Port (default ":9001")
  -fcgi-sock string
        FastCGI Socket (default "/tmp/go.sock")
  -http-addr string
        Set the HTTP bind address (default ":8080")
  -term-wait int
        How long to wait between SIGTERM(docker stop) and exit
```
# References
https://gist.github.com/hgfischer/7965620

