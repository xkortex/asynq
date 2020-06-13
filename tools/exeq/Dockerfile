# Builds exeq and asyq as static binaries
FROM golang:1.14.4-alpine as build

RUN  apk add --update --no-cache \
        bash \
        curl \
        ca-certificates \
        git \
        jq \
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
    &&  CGO_ENABLED=0 go build -o /usr/local/bin/asynq

RUN     cd $GOPATH/src/github.com/hibiken/asynq/tools/exeq \
    &&  CGO_ENABLED=0 go build -o /usr/local/bin/exeq

FROM scratch

COPY --from=build /usr/local/bin/asynq /asynq
COPY --from=build /usr/local/bin/exeq /exeq

ENTRYPOINT ["/asynq"]
