FROM golang:1.13.1 as bd
RUN adduser --disabled-login --gecos "" appuser
WORKDIR /github.com/layer5io/meshery-nsx-sm
ADD . .
RUN GOPROXY=direct GOSUMDB=off go build -ldflags="-w -s" -a -o /meshery-nsx-sm .
RUN find . -name "*.go" -type f -delete; mv nsx-sm /
RUN wget -O /nsx-sm.tar.gz https://github.com/nsx-sm/nsx-sm/releases/download/1.3.0/nsx-sm-1.3.0-linux.tar.gz

FROM alpine
RUN apk --update add ca-certificates
RUN mkdir /lib64 && ln -s /lib/libc.musl-x86_64.so.1 /lib64/ld-linux-x86-64.so.2
COPY --from=bd /meshery-nsx-sm /app/
COPY --from=bd /nsx-sm /app/nsx-sm
COPY --from=bd /nsx-sm.tar.gz /app/
COPY --from=bd /etc/passwd /etc/passwd
ENV nsx-sm_VERSION=nsx-sm-1.3.0
USER appuser
WORKDIR /app
CMD ./meshery-nsx-sm
