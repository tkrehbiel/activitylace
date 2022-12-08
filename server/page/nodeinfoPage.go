package page

// Serving /.well-known/nodeinfo

var WellKnownNodeInfo = StaticPage{
	Path:        "/.well-known/nodeinfo",
	Accept:      "*/*",
	ContentType: "application/json",
	Template: `
{
	"links": [
		{
			"rel": "http://nodeinfo.diaspora.software/ns/schema/2.1",
			"href": "{{ .URL }}/nodeinfo/2.1"
		}
	]
}`,
}

var NodeInfo = StaticPage{
	Path:        "/.well-known/nodeinfo/2.1",
	Accept:      "*/*",
	ContentType: "application/json",
	Template: `
{
	"version": "2.1",
	"software": {
		"name": "activitylace",
		"version": "0.0",
		"repository": "https://github.com/tkrehbiel/activitylace/",
		"homepage": "https://github.com/tkrehbiel/activitylace/"
	},
	"protocols": ["activitypub"]
}`,
}
