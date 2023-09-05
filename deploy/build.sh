#!/bin/sh
APP_NAME=intertube  

TAR_NAME=${APP_NAME}.zip

# clean up dist directory
if [ -f "${TAR_NAME}" ]; then
  rm ${TAR_NAME} deploydate
fi

date +%s > deploydate

GOOS=linux GOARCH=amd64 CGO_ENABLED=0 go build -o main -tags lambda
zip ${TAR_NAME} main
zip -ur ${TAR_NAME} assets
zip -u ${TAR_NAME} deploydate config.toml
