# Builds exeq and asyq as static binaries
ARG TARGET=exeq
FROM golang:1.14.4-alpine as build

RUN  apk add --update --no-cache \
        bash \
        curl \
        ca-certificates \
        git \
        jq \
        make \
        tar

# caching for speed
RUN     git clone https://github.com/hibiken/asynq.git $GOPATH/src/github.com/hibiken/asynq \
    &&  cd $GOPATH/src/github.com/hibiken/asynq \
    &&  go get

COPY . $GOPATH/src/github.com/hibiken/asynq

RUN     cd $GOPATH/src/github.com/hibiken/asynq \
    &&  go get \
    &&  cd $GOPATH/src/github.com/hibiken/asynq/tools/asynq \
    &&  go get \
    &&  cd $GOPATH/src/github.com/hibiken/asynq/tools/exeq \
    &&  go get

RUN     cd $GOPATH/src/github.com/hibiken/asynq/tools/asynq \
    &&  CGO_ENABLED=0 go build -o $GOPATH/bin/asynq

RUN echo hi
RUN     cd $GOPATH/src/github.com/hibiken/asynq/tools/exeq \
    &&  make get \
    &&  make \
#    && go get \
#    && cat main.go \
#    &&  CGO_ENABLED=0 go build -i -ldflags="-X 'main.Version=${VERSION}'" -o ${GOPATH}/bin/exeq \
    &&  /go/bin/exeq --version

## ========= give each binary their own container entrypoint  ==========
## === === === === switch debug dev === === === === ##
FROM build AS final-dev


## === === === === switch asynq === === === === ##
FROM scratch AS final-asynq

COPY --from=build /go/bin/asynq /asynq
ENTRYPOINT ["/asynq"]

## === === === === switch exeq === === === === ##
FROM scratch AS final-exeq

COPY --from=build /go/bin/exeq /exeq
ENTRYPOINT ["/exeq"]

## === === === === final === === === === ##
FROM final-${TARGET} as final

