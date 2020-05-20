#!/bin/sh
set -eu

mkdir -p /ssl
echo "$PLAYWITHGODEV_CERT_FILE" > /ssl/cert.pem
echo "$PLAYWITHGODEV_KEY_FILE" > /ssl/key.pem

exec nginx -g "daemon off;"
