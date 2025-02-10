## Steps to run the project

1. Install go
2. Run `go run ./...`
3. Send a post `http://localhost:8080/shorten` with a json body `{"url": "https://www.google.com"}` you'll get a json response with the short code
4. Open `http://localhost:8080/redirect?code=<short_code>` in your browser and you will be redirected to the original url

## Notes

- The project is using sqlite, so you don't need to install any database
