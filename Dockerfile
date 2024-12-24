# Use Go 1.20 bookworm as base image
FROM golang:1.20-bookworm AS base

USER root
# Move to working directory /build
WORKDIR /build

# Copy the go.mod and go.sum files to the /build directory
COPY go.mod go.sum ./

# Install dependencies
RUN go mod download

# Copy the entire source code into the container
COPY . .

# Build the application
RUN go build -o ondict

# Document the port that may need to be published
EXPOSE 1345

# Start the application
CMD ["/build/ondict","-serve","-f=md", "-e=mdx","-listen=0.0.0.0:1345"]
# CMD ["/bin/bash"]

