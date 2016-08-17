DEV_VER=0.1

default: compile

install-deps:
	go get -u github.com/kardianos/govendor
	if [ ! -d vendor ]; then govendor init ; else	govendor sync; fi

install-test-deps:
	go get -u github.com/golang/lint/golint
	go get golang.org/x/tools/cmd/cover
	go get github.com/onsi/ginkgo/ginkgo
	go get github.com/onsi/gomega

test:
	make install-deps install-test-deps
	go vet ./...
	golint ./...
	ginkgo -r -trace -failFast -v --cover --randomizeAllSpecs --randomizeSuites -p
	echo "" && for i in $$(ls **/*.coverprofile); do echo "$${i}" && go tool cover -func=$${i} && echo ""; done
	echo "" && for i in $$(ls **/**/*.coverprofile); do echo "$${i}" && go tool cover -func=$${i} && echo ""; done

# Make compilation depend on the docker dev container
# Run the build in the dev container leaving the artifact on completion
# Use run-dev to get an interactive session
docker-compile: dev
	docker run -i --rm --net host -v ~/.bash_history-dcos-tests:/root/.bash_history -v `pwd`:/go/src/github.com/adobe-platform/dcos-tests -w /go/src/github.com/adobe-platform/dcos-tests -e version=0.0.1  -e CGO_ENABLED=0 -e GOOS=linux -t f4tq/dcos-tests:dev make compile

compile: dev
	@echo "Compiling dcos-tests ..."
	@if [  -e /.dockerinit ]; then \
		CGO_ENABLED=0 GOOS=linux  go build .;\
	else \
		docker run -i --rm -e  CGO_ENABLED=0 -e GOOS=linux  -v `pwd`:/go/src/github.com/f4tq/dcos-tests -t f4tq/dcos-go-build:dev make install-deps compile ; \
	fi


build-container: compile
	@echo "Building dcos-tests container ..."
	@if [ ! -e /.dockerinit ]; then \
		docker build --tag f4tq/dcos-tests:`git rev-parse HEAD` .; \
	else \
		echo "You're in a docker container. Leave to run docker" ;\
	fi

upload-current:
	@set -x ; REV=`git rev-parse HEAD`; \
	if [ 0 -eq $$(docker images | grep 'f4tq/dcos-tests' | grep -c "$$REV") ]; then \
		make build-container ; \
	fi ; \
	docker tag f4tq/dcos-tests:$$REV f4tq/dcos-tests:`cat VERSION` ;\
	docker push f4tq/dcos-tests:`cat VERSION` 

build: compile

# build the docker dev container if it doesn't exists
dev:
	@set -x; if [ ! -e /.dockerinit ]; then \
	  (docker images | grep 'f4tq/dcos-tests' | grep -q dev) || \
	  docker build -f Dockerfile-dev -t f4tq/dcos-tests:dev . ; \
	fi

# run a shell in the docker dev environment, mounting this directory and establishing bash_history in the container instance
#run-dev: dev
##       save bash history in-between runs...
#	@if [ ! -f ~/.bash_history-dcos-tests ]; then touch ~/.bash_history-dcos-tests; fi
##       mount the current directory into the dev build
#	docker run -i --rm --net host -v ~/.bash_history-dcos-tests:/root/.bash_history -v `pwd`:/go/src/github.com/f4tq/dcos-tests -w /go/src/github.com/f4tq/dcos-tests -t f4tq/dcos-tests:dev bash


