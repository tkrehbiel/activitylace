package page

// ActorEndpoint is a template for an ActivityPub Actor endpoint
var ActorEndpoint = StaticPage{
	Path:        "", // must be set for each actor
	Accept:      "application/(activity|ld)+json",
	ContentType: "application/activity+json",
	Template: `
{
	"@context": [
      "https://www.w3.org/ns/activitystreams",
      "https://w3id.org/security/v1"
  	],
	"type": "{{ .UserType }}",
	"id": "{{ .UserID }}",
	"url": "{{ .UserProfileURL }}",
	"inbox": "{{ .InboxURL }}",
	"outbox": "{{ .OutboxURL }}",
	"followers": "{{ .FollowersURL }}",
	"following": "{{ .FollowingURL }}",
	"name": "{{ .UserDisplayName }}",
	"preferredUsername": "{{ .UserName }}",
	"manuallyApprovesFollowers": true,
    "publicKey": {
        "id": "{{ .UserPublicKeyID }}",
        "owner": "{{ .UserID }}",
        "publicKeyPem": "{{ .TransformedPublicKey }}"
    },
	"summary": "{{ .UserSummary }}"
	{{- if .AvatarURL -}},
	"icon": {
		"type": "Image",
		"url": "{{ .AvatarURL }}",
		"width": {{ .AvatarWidth }},
		"height": {{ .AvatarHeight }}
	}
	{{- end }}
}`,
}

var ProfilePage = StaticPage{
	Path:        "/profile/{username}", // must be set for each actor
	Accept:      "*/*",
	ContentType: "text/html",
	Template: `
<html>
<head>
<title>{{ .UserDisplayName }}</title>
</head>
<body>
<h1>{{ .UserDisplayName }}</h1>
<p>Latest activity from this account</p>
<ul>
	{{ range .LatestNotes }}
	<li><a href="{{ .URL }}">{{ .Content }}</a></li>
	{{ end }}
</ul>
</body>
</html>`,
}
