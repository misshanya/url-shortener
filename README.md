# URL Shortener

A simple service that shortens URLs.
My first practice with gRPC.

## Architecture

### Main service (shortener), gRPC

To shorten URL, this service stores original URL in PostgreSQL and encodes ID into base62.

To unshorten URL, it simply decodes base62 and gets original URL by id from DB.

### Gateway, REST

This service communicates with the `shortener` by gRPC.

To shorten URL, it gets base62 encoded id from `shortener` and constructs final URL using `PUBLIC_HOST` and base62. For example, base62 is `1z`, PUBLIC_HOST is `https://sh.some/`. Final URL is `https://sh.some/1z`.

To unshorten URL, it queries the `shortener` and gets original URL by base62 in the path param in the request. Then, it redirects with 302 to the original URL.

### Bot, Telegram inline mode

This service also communicates with the `shortener` by gRPC.

Bot only shortens the URL, unshortening process is on `gateway`.

Shortening process is the same as the one in `gateway`: walk to the `shortener`, get base62, connect public host.

It takes the URL in inline mode. For example, `@mybot https://github.com/misshanya/url-shortener`. And you will get the shortened URL.

## How to

### Prerequisites

- Docker
- Go 1.24+ (if you want to build without Docker)
- Telegram bot token

### Run

Clone repo and `cd` into it

```shell
git clone https://github.com/misshanya/url-shortener
cd url-shortener
```

Copy .env.example to the .env file and edit it with your preferences (like public url and bot token)

```shell
cp .env.example .env
```

Run!

```shell
docker compose up -d
```

### Gateway's usage

**Shorten** - `POST /shorten` with the following body:

 ```json
 {
   "url": "your-url-to-shorten"
 }
 ```

**Unshorten** - `GET /{base62}`

## License

This project is licensed under the MIT license. See the [LICENSE](./LICENSE) file for details.
