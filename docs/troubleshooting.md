Problems I encountered along the way.

## Pleroma gives up after doing a Webfinger

Pleroma is picky about the Content-Type response from the webfinger page. If you don't set the Content-Type to either json or xml, Pleroma ignores you. [Source](https://git.pleroma.social/pleroma/pleroma/-/blob/develop/lib/pleroma/web/web_finger.ex#L205)

Resolution: Make sure the Content-Type of the webfinger json response is set to `application/json` or `application/jrd+json`. I think technically it should default to interpreting it as json, but whatever.

## Mastodon says "503 Remote SSL certificate could not be verified"

[Webfinger.net](https://webfinger.net) reports "x509: certificate signed by unknown authority"

`openssl s_client -connect domain.name:443` confirms the lack of a certificate chain: `verify error:num=20:unable to get local issuer certificate`

(Weirdly, Pleroma doesn't report the error, I guess it isn't very strict about certificates.)

Resolution: Was using Let's Encrypt's `cert.pem` instead of `fullchain.pem` in the nginx config. It should be:

```
ssl_certificate /etc/letsencrypt/live/___/fullchain.pem;
ssl_certificate_key /etc/letsencrypt/live/___/privkey.pem;
```

## Mastodon gives up after Webfinger too

Resolution: This was caused by [misspelling (actually mis-casing) `preferredUsername`](https://github.com/tkrehbiel/activitylace/commit/8efbefeec5b58cc7e5750a40c6a98d9f62179f10) in the Actor object response. It's case-sensitive. Mastodon takes the `preferredUsername` as canonical, and if it's misspelled, it tries to use an empty string as the username and everything blows up.
