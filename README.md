This is the source code for inter.tube, [as seen on HN's "Stripe killed my music locker service, so I'm open sourcing it"](https://news.ycombinator.com/item?id=36403607). inter.tube is an online music storage locker service with Subsonic API support.

Note that none of this code was intended to be seen by anyone else, so it's rough, but I hope it is useful to someone. I was inspired to open source it by the recent Apollo debacle.

#### Stripe Update

I heard back from a Stripe employee and it turns out this service _is_ OK to host! inter.tube won't die, but it will remain open source.

### Architecture

This uses Backblaze B2 to host files. It uses Cloudflare Workers to access B2 so that bandwidth is free.

The backend itself is Go, using SSR (html/template) and some hairy vanilla JS for the browser music player. It runs on AWS Lambda. The data is stored in DynamoDB. There is some functionality for caching user libraries as JSON blobs in S3 (via DynamoDB Stream event handling lambdas), but it's kind of a mess.

### Running it locally

Here's a way to run this easily, using DynamoDB local and MinIO:

```bash
docker compose up -d
go build
./intertube --cfg=config.example.toml
```

Then access the site at http://localhost:8000.

### Configuration

See `config.example.toml`.

You can specify the config file with the `--cfg file/path.toml` command line option.

By default it looks at `config.toml` in the working directory.

### Interested?

This project is not in a good place to self-host, but I'm open to working on it more or accepting contributions. Feel free to create an issue or discussion thread.
