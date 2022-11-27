package page

// ActorEndpoint is a template for an ActivityPub Actor endpoint
var ActorEndpoint = StaticPage{
	Path:        "", // must be set for each actor
	Accept:      "application/activity+json",
	ContentType: "application/activity+json",
	Template: `
{
	"@context": "https://www.w3.org/ns/activitystreams",
	"type": "{{ .UserType }}",
	"id": "{{ .UserID }}",
	"url": "{{ .UserProfileURL }}",
	"inbox": "{{ .InboxURL }}",
	"outbox": "{{ .OutboxURL }}",
	"followers": "{{ .FollowersURL }}",
	"following": "{{ .FollowingURL }}",
	"name": "{{ .UserDisplayName }}",
	"preferredUserName": "{{ .UserName }}",
	"manuallyApprovesFollowers": true,
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
<title>profile of {{ .UserName }}</title>
</head>
<body>
<p>profile of {{ .UserName }}</p>
</body>
</html>`,
}
