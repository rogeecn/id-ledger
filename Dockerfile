FROM golang:1.26-alpine AS build

WORKDIR /src
COPY go.mod go.sum ./
RUN go mod download
COPY cmd ./cmd
COPY internal ./internal
RUN CGO_ENABLED=0 go build -trimpath -ldflags="-s -w" -o /out/id-ledger ./cmd/id-ledger

FROM alpine:3.22

RUN addgroup -S id-ledger && adduser -S -G id-ledger id-ledger \
    && mkdir /data && chown id-ledger:id-ledger /data
COPY --from=build /out/id-ledger /usr/local/bin/id-ledger

USER id-ledger
EXPOSE 3000
ENV ID_LEDGER_ADDR=:3000 \
    ID_LEDGER_DATABASE_PATH=/data/id-ledger.db
VOLUME ["/data"]
ENTRYPOINT ["id-ledger"]
