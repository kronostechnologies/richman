FROM golang:1.13 AS builder
WORKDIR /go/src/github.com/kronostechnologies/richman/
COPY . .
RUN CGO_ENABLED=0 EXTRA_LDFLAGS="-w -s" make compile

FROM scratch
COPY --from=builder /go/src/github.com/kronostechnologies/richman/bin/richman /bin/
ENTRYPOINT ["/bin/richman"]