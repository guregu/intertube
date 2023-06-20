This is the source code for inter.tube, killed by Stripe's policies. inter.tube is (was) an online music storage locker service with Subsonic API support.

### Architecture

This uses Backblaze B2 to host files. It uses Cloudflare Workers to access B2 so that bandwidth is free.

The backend itself is Go, using SSR (html/template) and some hairy vanilla JS for the browser music player. It runs on AWS Lambda. The data is stored in DynamoDB. There is some functionality for caching user libraries as JSON blobs in S3 (via DynamoDB Stream event handling lambdas), but it's kind of a mess.

### Environment variables

B2 is Backblaze B2, CF is Cloudflare.

```bash
export B2_KEY_ID=
export B2_KEY=
export CF_ACCOUNT=
export CF_API_EMAIL=
export CF_API_KEY=
export CF_KV_NAMESPACE=

# Stripe stuff, but don't bother...
export STRIPE_ACCOUNT=
export TEST_STRIPE_PUBLIC=
export TEST_STRIPE_KEY=
export TEST_STRIPE_SIG=
export STRIPE_PUBLIC=
export STRIPE_KEY=
export STRIPE_SIG=
```

Unfortunately, there's some hardcoded bucket names and domains that need to be made configurable.

### Interested?

This project is not in a good place to self-host, but I'm open to working on it more or accepting contributions. Feel free to create an issue or discussion thread.
