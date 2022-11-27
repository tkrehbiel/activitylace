package page

var WellKnownHostMeta = StaticPage{
	Path:        "/.well-known/host-meta",
	Accept:      "*/*",
	ContentType: "application/xml",
	Template: `
<?xml version="1.0" encoding="UTF-8"?>
<XRD xmlns="http://docs.oasis-open.org/ns/xri/xrd-1.0">
	<Link rel="lrdd" template="{{ .URL }}/.well-known/webfinger?resource={uri}"/>
</XRD>`,
}
