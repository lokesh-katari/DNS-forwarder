# DNS Forwarder with Redis Cache

## Overview

A simple and efficient DNS forwarder written in Go.
Forwards DNS queries to upstream servers while caching responses in Redis for faster subsequent lookups.
Aims to improve DNS performance and reduce load on upstream servers.
## Features

Redis Caching: Stores DNS responses in Redis for rapid retrieval.
Upstream Server Support: Forwards queries to multiple upstream DNS servers.
Customizable Configuration: Adjust settings for DNS servers, Redis connection, and caching behavior.
Error Handling and Logging: Provides meaningful error messages and logs for troubleshooting.
## Installation

### Prerequisites:
  Go (version 1.18 or later)
  Redis
  Install Dependencies:
  ```Bash
go get ./...
  ```
Build the Project:
  ```Bash
go build
  ```

## Usage

### Configure:
Set environment variables or edit a configuration file (if available) to specify:
Upstream DNS servers
Redis connection details
Caching options
Run the Forwarder:
  ``` Bash
./dns-forwarder
  ```
## Configuration Options

DNS_SERVERS: A comma-separated list of upstream DNS servers (e.g., "8.8.8.8,1.1.1.1").
REDIS_HOST: Redis hostname or IP address.
REDIS_PORT: Redis port (default: 6379).
CACHE_TTL: TTL for cached responses in seconds (default: 300).
## Contributing

Fork the repository.
Create a branch for your changes.
Make your changes with clear commit messages.
Submit a pull request.

# DEMO


https://github.com/lokesh-katari/DNS-forwarder/assets/111894942/567a74e7-93dc-4b1a-8e54-3768f212e86a

