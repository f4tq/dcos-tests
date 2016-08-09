FROM       golang:1.6.2-alpine
MAINTAINER Adobe <fortescu@adobe.com>

ADD dcos-tests  /usr/local/bin/

CMD /usr/local/bin/dcos-tests 
