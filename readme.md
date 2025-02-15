## Steps to run the project

1. Install [go](https://go.dev/dl/)
2. Run `go run ./...`
3. Send a post `http://localhost:8080/shorten` with a json body `{"url": "https://www.google.com"}` you'll get a json response with the short code
4. Open `http://localhost:8080/redirect?code=<short_code>` in your browser and you will be redirected to the original url

## Notes

- The project is using sqlite, so you don't need to install any database

## Load testing

### 10 concurrent requests in a second

For /shorten
Response time distribution:
- p50 1.5ms
- p90 2.7ms
- p95 2.7ms
- p99 2.7ms

For /redirect
- p50 174.5ms
- p90 181ms
- p95 181ms
- p99 181ms

### 50 concurrent requests in a second

For /shorten
- p50 0.9ms
- p90 1.4ms
- p95 1.4ms
- p99 2.8ms

For /redirect
- p50 179.5ms
- p90 192.5ms
- p95 200ms
- p99 415.1ms

### 100 concurrent requests in a second

For /shorten
- p50 1.3ms
- p90 1.6ms
- p95 1.7ms
- p99 3.7ms

For /redirect
- p50 176.1ms
- p90 182.6ms
- p95 186.1ms
- p99 189.5ms

### 200 concurrent requests in a second 

For /shorten
- p50 0.9ms
- p90 1.1ms
- p95 1.2ms
- p99 2.2ms

For /redirect (success rate 99%)
- p50 190.8ms
- p90 198.2ms
- p95 203.2ms
- p99 211.7ms

### 500 concurrent requests in a second 

For /shorten (Success rate:	73.6%)
- p50 0.6ms
- p90 0.8ms
- p95 0.9ms
- p99 1.2ms

For /redirect (success rate 47.2%)
- p50 295.9ms
- p90 405.9ms
- p95 411.9ms
- p99 414.8ms

### 1000 concurrent requests in a second 

For /shorten (Success rate:	55.90%)
- p50 0.4ms
- p90 0.7ms
- p95 0.8ms
- p99 2.2ms

For /redirect (success rate 23%)
- p50 339.5ms
- p90 431.7ms
- p95 434.2ms
- p99 438.0ms

<!-- oha -c 10 -q 10 -n 10 -m POST http://localhost:8080/shorten -H "Content-Type: application/json" -d '{"url":"https://www.google.com"}' -->

<!-- oha -c 1300 -q 1300 -n 1000 -m POST http://localhost:8080/redirect?code=rGu2aeQO -->
