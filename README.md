# URL Shortener

URL Shortener that just shorts URL.
My first practice with gRPC.

## Architecture

### Main service (shortener), gRPC

To short URL, this service calculates SHA256 hash and stores it's first characters in PostgreSQL.

To unshort URL, it simply walks to the DB and gets original URL by hash

### Gateway, REST

This service communicates with the `shortener` by gRPC.

To short URL, it gets hash from `shortener` and connects `PUBLIC_HOST` with hash. For example, hash is `asdf`, PUBLIC_HOST is `https://sh.some/`. Final URL is `https://sh.some/asdf`.

To unshort URL, it walks to the `shortener` and gets original URL by hash in the path param in the request. Then, it redirects with 302 to the original URL.

### Bot, Telegram inline mode

This service also communicates with the `shortener` by gRPC.

Bot only shorts the URL, unshorting process is on `gateway`.

Shorting process is same as the `gateway`'s one: walk to the `shortener`, get hash, connect public host.

It tooks the URL in inline mode. For example, `@mybot https://github.com/misshanya/url-shortener`. And you will get the shortened URL.

## How to

### Prerequisites

- Docker
- Go 1.24+
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

---

**Short** - `POST /shorten` with the following body: 
 ```json
 {
   "url": "your-url-to-short"
 }
 ```

---

**Unshort** - `GET /{hash}`

---

## License

This project is licensed under the MIT license. See the [LICENSE](./LICENSE) file for details.
