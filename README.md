# Assistant Proxy

This project provides a simple reverse proxy for OpenAI-compatible APIs with a custom `/v1/responses` endpoint. All requests are forwarded to the configured target API except `/v1/responses`, which stores conversation history using a configurable backend and forwards the conversation to the target chat completion endpoint.

## Configuration

Copy `.env.example` to `.env` and adjust the values:

- `TARGET_API_URL` – base URL of the API
- `TARGET_API_KEY` – API key to forward in requests
- `MEMORY_TYPE` – `sqlite` or `redis`
- `SQLITE_PATH` – path for the SQLite database
- `REDIS_ADDR` – address of the Redis server

## Build

```sh
go build
```

## Usage

Run the binary after creating the `.env` file. The proxy listens on `:8080`.
