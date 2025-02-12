# Start from base image
FROM golang:1.24-alpine as build

# Set the current working directory inside the container
WORKDIR /app

# Copy go mod and sum files
COPY go.mod go.sum ./

# Download all dependencies
RUN go mod download

# Copy source from current directory to working directory
COPY . .

# Build the application
# Produce binary named main
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -ldflags="-s -w" -a -v ./...

########################

# Start from a new lightweight image
FROM alpine:latest

# Set the working directory inside the container
WORKDIR /app

# Copy the binary from the build image to the working directory
COPY --from=build /app/main .

# Run the binary
ENTRYPOINT ["./main"]