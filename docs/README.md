# Project Plan

This is the project plan, of sorts, in the form of loosely-organized user stories and acceptance criteria. (Before Agile, we just called this kind of thing a todo list.)

The initial goal is to create an ActivityPub backend that works in conjunction with an existing blog, possibly running on the same server but it shouldn't have to. This will allow fediverse actors to follow the blog as they would any other actor. (Probably exposed as an Organization instead of a Person.)

If this doesn't run on the same server, the blog server will have to redirect webfinger and nodinfo requests to the ActivityPub server.

## Phase 1, Outbox

- Written in Golang, which happens to be good at web-based services, and I happen to know how to do it. The down side is installing it will be more complicated, putting it out of reach of anyone who doesn't know how to setup and run a VPS or cloud server. A problem for another day.
  - I _do not_ want to write it in PHP for a LAMP stack. LAMP is dead. Let it die.
  - I don't know Node.js well enough to attempt that, but that might be a decent choice.
- ~~Supports any existing blog that has an RSS or Atom feed, possibly even one of them new-fangled JSON feeds.~~ (done, though not tested with very many feeds)
- ~~Server will expose nodeinfo and webfinger endpoints.~~ (done)
- ~~Server will expose an Organization actor endpoint representing the blog.~~ (done)
- ~~Server will expose an Outbox that processes HTTP GET requests.~~ (done)
- ~~Server will initially expose an Inbox that ignores input.~~ (done)
- A link to the blog's RSS feed will essentially be the data source for the blog actor's Outbox.
  - ~~The server will need to periodically fetch the blog feed to check for new items.~~ (done) Every five minutes should be sufficient, but it should be configurable.
  - ~~_Should_ use an HTTP HEAD to check if the page is modified before fetching the full feed. I used to know how to do that, but will probably have to look it up again to refresh my memory.~~ [In fact it uses HTTP GET with If-Modified-Since.] (done)
- Storage of ActivityPub objects should be accomplished with minimal resources. I don't want to require installing a full database, at least initially.
  - ~~After some experiments, I think a file-based sqlite backend is probably the simplest way to go. Was going to try to use json files, but there's too much complexity to manage.~~ (done)
  - Keep in mind we will probably need a system to purge older items from the database periodically or else it will keep growing forever. A system to export items marked for deletion to external archival storage may be needed.

## Phase 2, Followers

- The blog actor's Inbox will receive follow POST requests.
- The blog actor's Outbox will notify followers of new posts by POSTing to remote servers. (See: Rate-limited output messaging pipeline.)
- May require POST requests to be signed in some way to avoid spam. Need to investigate how that works. (I don't think there's a standard for this in ActivityPub.)
- Follows from blocked actors will be discarded.
- Follows from blocked servers will be discarded.
- To protect against the unlikely scenario of a million followers appearing out of nowhere, impose a limit on the number of followers for now. Say, 1000.

## Phase 3, Replies
- The blog actor's Inbox will receive activity Notes (replies) and treat them as blog comments. Some way to filter out private replies might be needed. Not sure how that works yet. I think ActivityPub notes are tagged as being copied to "public"?
- Replies from blocked actors will be discarded.
- Replies from blocked servers will be discarded.
- Replies from hidden actors or servers will be received, but not displayed without some kind of opt-in.
- Server will expose a static HTML page of received replies for any given blog permalink, so the blog can embed the page into their posts as they would any other comment backend.
- Replies from blocked or hidden actors or servers will not be displayed. A hidden flag can be used to hide objectionable content behind some kind of opt-in click. It can also be a sort of "timeout" period to decide whether to block the actor.
- Server may expose a JSON list of received replies for any given blog permalink, so blogs can render the comments in any way they choose.
- Server may expose an RSS feed of received replies for any given blog permalink, so readers outside the fediverse can follow them.
- Server will not expose any way for users to create comments outside the fediverse, at least initially. That would require managing client identities and authentication, and that's hard.

## Phase 4, Persons

Future goals will be to expose one or more Person actors associated with the blog author(s). That's harder because it requires client authentication and authorization, and a way to store created notes that's easy to read and export for archival storage.

Some consideration for scaling. For example, imagine a world where the blog gets one million followers. That would mean sending out _one million_ individual ActivityPub notifications on every blog post. That would most assuredly require some kind of rate-limited output pipeline running as a separate process. I don't know if ActivityPub supports grouping notifications by destination server, but it would be nice if it did. (Perhaps one Note with a long To or BTo list? Possible privacy concerns there though. Receiving server will know everyone who is following the blog by just looking at the list.)

## Subsystems

- Configuration. Config will be by editing config files on the server at first. Otherwise, account management and authentication will be needed to expose an admin HTML page.
- Static Page Rendering. Most ActivityPub endpoints (webfinger, nodeinfo, person, following, followers) can be rendered as static pages.
- Dynamic Page Rendering. But some ActivityPub endpoints (inbox, outbox) will need to be dynamically rendered. But should support some kind of caching. No need to re-calculate a page more than, say, once a minute.
- Storage. To persist a collection of ActivityPub objects. I want to store incoming ActivityPub objects exactly as they arrive, which makes it a little more complicated to serialize and deserialize objects which may have unknown properties.
  - Storage should be somewhat agnostic about the format it reads and writes.
  - It should eventually be able to support any database backend, relational or nosql, for future scaling (though I don't expect to have to scale that much, unless a million followers appear).
  - It should support purging old objects, perhaps moving them to offline archival storage.
- RSS. A system to poll an RSS feed periodically to find new items, render them as ActivityPub notes, and store them.
- Caching. ActivityPub objects should be cached for a period of time.
- Authentication and Authorization. Not needed so much at first, but will eventually be needed, so something to keep in mind. All requests should pass through some kind of authorization function to be determined later.
- Rate-limited sending pipeline system. Probably not needed initially but give thought to adding it in the future, i.e. build a framework for it even if it isn't implemented yet. It _will_ be needed someday.
