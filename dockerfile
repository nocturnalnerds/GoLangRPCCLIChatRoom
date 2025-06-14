
FROM golang:1.21-alpine

WORKDIR /app

COPY go.mod ./
COPY go.sum ./

RUN go mod download

COPY . .

RUN go build -o client client.go

EXPOSE 50045

# Run the server binary
CMD ["./client"]