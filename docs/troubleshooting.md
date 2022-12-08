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

Resolution: This was caused by [misspelling (actually mis-casing) `preferredUsername`](https://github.com/tkrehbiel/activitylace/commit/8efbefeec5b58cc7e5750a40c6a98d9f62179f10) in the Actor object response. It's case-sensitive. Mastodon takes the `preferredUsername` as canonical, and if it's misspelled or presumably missing, it tries to use an empty string as the username and everything blows up.

Note: `preferredUsername` is NOT a required field in ActivityPub. So right off the bat, Mastodon is not ActivityPub compliant.

## Implementation Note

11/29/2022 After some thought, I think it's a mistake to try to represent ActivityPub objects internally _as_ native ActivityPub objects. The LSON-LD object format is ... weird. I think it will be easier to use an internal representation of actors, notes, follows, etc. and then translate them to and from ActivityPub when they are sent or received.

## Pleroma sends Follow but doesn't actually follow

Pleroma successfully sends a Follow activity to the inbox, but it doesn't list the remote account among its followers. Responding to the Follow activity with a 200 OK is apparently not enough.

Resolution: _Apparently_ you have to send an Accept activity back to the source before the Follow action is complete. I discovered this by searching the source code of an ActivityPub PHP WordPress plugin. I finally found this buried in the spec [here](https://www.w3.org/TR/activitypub/#accept-activity-inbox). Soapbox: ActivityPub is poorly documented for a W3C standard.

## Race Condition sending Accept back after a Follow request

Server kept hanging when trying to send an Accept back to the origin after receiving a Follow request.

Resolution: Oh! I'm dumb. This is a golang issue. I was sending out a new web request (and trying to wait for it to return) in the middle of the follow web handler. So I need to queue the Accept to run at a later time. Duh.

## Pleroma returns 400 from Accept

On sending the Accept activity to the remote server after receiving a Follow activity, HTTP errors are returned. Pleroma returns a 400 Bad Request, Mastodon returns a 401 Unauthorized.

Theory: Signatures not implemented? Doesn't seem to be the case for Pleroma. I see no error messages in the Pleroma log indicating invalid signatures. (I also see no errors clearly explaining why it failed.)

Pleroma [seems to require](https://git.pleroma.social/pleroma/pleroma/-/blob/develop/lib/pleroma/web/activity_pub/object_validators/accept_reject_validator.ex#L32) `to` and `cc` fields in the Accept object, which makes no sense whatsoever. In any case, including them didn't fix the 400 error.

Pleroma _apparently_ fetches the remote following collection in the process of its Follow logic??

```
Dec  3 03:34:42 localhost pleroma: request_id=Fy0rJt2ffaarr_8AloWB [debug] Fetching object https://user/following via AP
Dec  3 03:34:42 localhost pleroma: request_id=Fy0rJt2ffaarr_8AloWB [error] Follower/Following counter update for https://user failed.#012{:error, "Object has been deleted"}
Dec  3 03:34:42 localhost pleroma[167754]: 03:34:42.843 request_id=Fy0rJt2ffaarr_8AloWB [error] Follower/Following counter update for https://user failed.
```

Maybe a following/follower collection implemention is required? Still might be a signature though.

Update 12/6/2022: I implemented http signatures and it didn't help. Don't yet know what the problem is. Hard to debug.

## Mastodon returns 401 from Accept

Similar to Pleroma, on sending the Accept activity to the remote server after receiving a Follow activity, Mastodon returns a 401 Unauthorized.

12/6/2022 At first I thought it was because signatures weren't implemented. So I went ahead and implemented http signatures, but Mastodon still doesn't recognize them, and it returns the following error in the body: `Verification failed for [user@domain https://id] using rsa-sha256 (RSASSA-PKCS1-v1_5 with SHA-256)`. It's great for me that it did, but it seems like it's a lot more information than it should be putting into an error response. Generally you don't want to expose too much of that over a public channel.

I'm using `github.com/go-fed/httpsig` to create and verify signatures. It successfully _verifies_ signatures from Mastodon (and Pleroma). I did an independent functional test with a web site https://dinochiesa.github.io/httpsig/ which verifies http signatures online, and my signatures seemed to work, although it wasn't possible to test `(request-header)`. I'm stumped. Will probably have to resort to reading [the Mastodon source code](https://github.com/mastodon/mastodon/blob/main/app/controllers/concerns/signature_verification.rb#L78). (It didn't help.)

## Mastodon and `@context https://w3id.org/security/v1`

Mastodon [gives the example](https://blog.joinmastodon.org/2018/06/how-to-implement-a-basic-activitypub-server/) of endpoints including the https://w3id.org/security/v1 context which I think is intended to define the `publicKey` extension, but [the actual spec](https://w3c.github.io/vc-data-integrity/vocab/security/vocabulary.html) does not define a publicKey block like Mastodon uses. The spec defines the `publicKey` as a URL to a key, not a block of metadata. So I'm not sure it makes sense to include https://w3id.org/security/v1 in the @context. Then again, it's almost impossible to figure out JSON-LD schemas.
