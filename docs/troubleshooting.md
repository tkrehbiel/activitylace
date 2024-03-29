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

This contains links that may help: https://www.drupal.org/project/activitypub/issues/3179629

12/11/2022 Still no luck. [Mastodon inexplicably says](https://docs.joinmastodon.org/spec/security/) "The signature string is then hashed with SHA256 and signed with the actor’s _public_ [sic] key." Signing with a public key makes no sense, because, you know, it's public.

It's not a typo because it then goes on to say: "This request is functionally equivalent to saying that https://my-example.com/actor is requesting https://mastodon.example/users/username/inbox and is proving that they sent this request by signing (request-target), Host:, and Date: _with their public key_ [sic] linked at keyId, resulting in the provided signature."

I think it's just a misunderstanding of how it works, because the code uses `keypair.sign` and surely the makers of Ruby know to sign with the private key instead of the public key.

I tried to use `rsa.SignPKCS1v15` instead of `privKey.Sign` and it fared no better or worse. I'm utterly baffled by Mastodon's refusal to accept these signatures. The only thing I can think of is it's reporting the wrong error to me. Maybe try an `hs2019` algorithm with SHA512???? (Which is technically more secure but I doubt anyone else in the fediverse will support it since everything including Mastodon seems to be hard-coded for rsa-sha256.)

## Mastodon and `@context https://w3id.org/security/v1`

Mastodon [gives the example](https://blog.joinmastodon.org/2018/06/how-to-implement-a-basic-activitypub-server/) of endpoints including the https://w3id.org/security/v1 context which I think is intended to define the `publicKey` extension, but [the actual spec](https://w3c.github.io/vc-data-integrity/vocab/security/vocabulary.html) does not define a publicKey block like Mastodon uses. The spec defines the `publicKey` as a URL to a key, not a block of metadata. So I'm not sure it makes sense to include https://w3id.org/security/v1 in the @context. Then again, it's almost impossible to figure out JSON-LD schemas.

## Mastodon doesn't recognize a webfinger account

Mastodon is very picky that the webfinger `rel=self` link is identified with type `application/activity+json` instead of `application/ld+json` (which is the actual type of the ActivityPub document at that url).

```
	"links": [
		{
			"rel": "self",
			"type": "application/activity+json",
			"href": "__"
		}
    ]
```

NOT:

```
"type": "application/ld+json",
```

ActivityPub is quite clear that `application/ld+json` support is required and `application/activity+json` is optional. But then webfinger or any kind of remote discovery isn't part of ActivityPub anyway.

## Pixelfed and Friendica Accepted

12/11/2022 For both Follow and Undo Follow, Pixelfed responds with status 200 to the signed Accept activity. For both Follow and Undo Follow, Friendica responds with status 202 to the signed Accept activity.

Pixelfed: `Signature: keyId="individual user#main-key",headers="(request-target) date host accept digest content-type user-agent",algorithm="rsa-sha256",signature="..."`

Friendica: `Signature: keyId="server instance#main-key",algorithm="rsa-sha256",headers="(request-target) date host",signature="..."`

Pixelfed does not seem to require a signature for an Accept activity, although I see signature verification code [in the codebase](https://github.com/pixelfed/pixelfed) (PHP, shudder). Friendica also doesn't seem to require a signature for an Accept activity.

For comparison (these don't yet work):

Pleroma: `Signature: keyId="server instance#main-key",algorithm="rsa-sha256",headers="(request-target) date host",signature="..."`

Mastodon: `Signature: keyId="individual user#main-key",algorithm="rsa-sha256",headers="(request-target) host date digest content-type",signature="..."`

## Pixelfed and Friendica don't display any notes

12/12/2022 Both Pixelfed and Friendica seem to successfully Follow the server, and accept Create Note activities sent to their inbox, but I don't see them displayed anywhere.
