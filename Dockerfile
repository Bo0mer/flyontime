FROM golang:1.10
WORKDIR /go/src/github.com/Bo0mer/flyontime
ADD . .
RUN CGO_ENABLED=0 GOOS=linux go install -a -installsuffix cgo ./cmd/flyontime

FROM centurylink/ca-certs
WORKDIR /
COPY --from=0 /go/bin/flyontime .
CMD ["./flyontime"]  
