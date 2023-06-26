This is the source code for [inter.tube](https://inter.tube), [as seen on HN's "Stripe killed my music locker service, so I'm open sourcing it"](https://news.ycombinator.com/item?id=36403607) (spoilers: they didn't kill it after all). inter.tube is an online music storage locker service with Subsonic API support.

Note that none of this code was originally intended to be seen by anyone else, so it's rough, but I hope it is useful to someone. I was inspired to open source it by the recent Apollo debacle.

### Architecture

- Database: DynamoDB
- Storage: S3 or S3-compatible
- Backend: Go, server-side rendering + SubSonic API support
- Frontend: HTML and sprinkles of vanilla JS
- Runs as a regular webserver or serverless via AWS Lambda (serverless docs coming soon)

### Running it locally

Here's a way to run this easily, using DynamoDB local and MinIO.

Install these things:
- [Go compiler](https://go.dev/dl/) (latest version)
- Docker or equivalent

```bash
# git clone this project, then from the root directory:
docker compose up -d
go build
./intertube --cfg=config.example.toml
```

Then access the site at http://localhost:8000.

When running in local mode, you can edit the HTML templates and they should reload without having to restart the server.

### Running it on The Cloud

Docs coming soon :-)

### Configuration

See `config.example.toml`. It matches the `docker-compose.yml` settings.

You can specify the config file with the `--cfg file/path.toml` command line option.

By default it looks at `config.toml` in the working directory.

### Roadmap

- [x] inter.tube launch
- [x] Local dev mode
- [ ] Align latest changes with production
- [ ] Proper self-hosting guide
- [ ] ???
- [ ] Profit

### Contributing

Contributions, bug reports, and feature suggestions are welcome.

Please make an issue before you make a PR for non-trivial things.

You can sponsor this project on GitHub or buy an inter.tube subscription on the official site to help me out as well.
