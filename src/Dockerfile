FROM golang:1.26 AS build-go
WORKDIR /incident-service
COPY go.mod .
COPY go.sum .
RUN go mod download
COPY . .
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /bin/incident-tracking-service .

FROM alpine:latest
RUN addgroup -S incident && adduser -S incident -G incident
USER incident
WORKDIR /home/incident
COPY --from=build-go /bin/incident-tracking-service ./
EXPOSE 3000
ENTRYPOINT ["./incident-tracking-service"]
