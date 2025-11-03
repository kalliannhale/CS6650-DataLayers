# syntax=docker/dockerfile:1

# Declare the base image
FROM golang:1.25.1 AS build-stage

# Set destination for COPY
WORKDIR /app

# Download Go modules
COPY go.mod go.sum ./
RUN go mod download

# Copy the source code
COPY *.go ./

# Build the application
RUN CGO_ENABLED=0 GOOS=linux go build -o /online-store-mysql

# Deploy the application binary into a lean image
FROM gcr.io/distroless/base-debian11 AS build-release-stage
WORKDIR /
COPY --from=build-stage /online-store-mysql /online-store-mysql

# Document in the Dockerfile what ports the application is going to listen on by default.
EXPOSE 8080

# Use a non-root user to run the application
USER nonroot:nonroot

# Run the application
ENTRYPOINT ["/online-store-mysql"]