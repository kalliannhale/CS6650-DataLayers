# --- STAGE 1: Build ---
# Use an official Go image to build the application
FROM golang:1.25-alpine AS builder

# Set the working directory inside the container
WORKDIR /app

# Copy the go.mod and go.sum files
COPY go.mod . 
COPY go.sum .

# Download all the dependencies
RUN go mod download

# Copy your main.go file (and any other .go files)
COPY . .

# Build the application, creating a static binary.
# This is the "magic" that makes it small and fast.
RUN CGO_ENABLED=0 GOOS=linux GOARCH=amd64 go build -o /main .

# --- STAGE 2: Run ---
# Use a minimal "scratch" image for the final container.
# This is very secure and small.
FROM scratch

# Copy the static binary you just built
COPY --from=builder /main /

# Copy the SSL certificate authorities (so AWS SDK can make secure calls)
COPY --from=builder /etc/ssl/certs/ca-certificates.crt /etc/ssl/certs/

# Command to run the application when the container starts
CMD ["/main"]
