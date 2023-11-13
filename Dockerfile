FROM golang:1.21.4-alpine AS build 

WORKDIR /app

COPY go.mod ./
COPY go.sum ./
RUN go mod download

COPY *.go ./
RUN go build -o druid-tasks-exporter

FROM scratch
WORKDIR /
COPY --from=build /app/druid-tasks-exporter .
CMD ["/druid-tasks-exporter"]
