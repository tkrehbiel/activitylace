This directory contains a suite of curl payloads to test functionality.

e.g.
```
curl -v --request "POST" --header "Content-Type: application/ld+json" --data @payload.json http://localhost:8080/activity/test/inbox
```

Creating public/private keys for activitypub users:

```
openssl genrsa -out private.pem 2048
openssl rsa -in private.pem -outform PEM -pubout -out public.pem
```
