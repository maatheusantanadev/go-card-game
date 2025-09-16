FROM golang:1.22

WORKDIR /app

COPY go.mod ./
RUN go mod download

COPY . .

# Compila o servidor e o load_tester
RUN go build -o server ./cmd/server/main.go && \
    go build -o load_tester ./cmd/test/load_tester.go

# Expõe as portas do servidor
EXPOSE 4000
EXPOSE 4001/udp

CMD ["./server"]
