package rss

import (
	"bytes"
	"context"
	"fmt"
	"net/http"
	"net/http/httptest"
	"testing"
	"time"

	"github.com/mmcdole/gofeed"
	"github.com/stretchr/testify/assert"
	"github.com/stretchr/testify/mock"
	"github.com/stretchr/testify/require"
)

const firstRSS = `
<?xml version="1.0" encoding="utf-8" ?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Endgame Viable</title>
    <link>https://endgameviable.com/</link>
    <item>
      <title>ActivityPub And Me, Part 1 of ?</title>
      <link>https://endgameviable.com/dev/2022/11/activitypub-and-me-part-1/</link>
      <pubDate>Fri, 18 Nov 2022 13:25:34 -0500</pubDate>
      <guid>https://endgameviable.com/dev/2022/11/activitypub-and-me-part-1/</guid>
      <enclosure url="https://media.endgameviable.com/img/2019/08/html-header-image.jpg" length="0" type="image/jpeg" />
      <description>&lt;p&gt;I&amp;rsquo;ve been on a learning rampage on the topic of &lt;a href=&#34;https://en.wikipedia.org/wiki/Fediverse&#34;&gt;the fediverse&lt;/a&gt; lately, and there&amp;rsquo;s plenty of material for writing blog posts.&lt;/p&gt;
&lt;p&gt;With everyone &lt;a href=&#34;https://twitterisgoinggreat.com/#just-setting-up-their-quitters&#34;&gt;freaking out about Twitter again today&lt;/a&gt;, it seems like I good time to publish this draft. However, I&amp;rsquo;m specifically &lt;em&gt;not&lt;/em&gt; going to:&lt;/p&gt;
&lt;p&gt;Anyhoo, those are some of my ActivityPub interests and the obstacles I need to overcome to make it happen.&lt;/p&gt;</description>
    </item>
	<item>
	  <title>PC Gaming Wasteland</title>
	  <link>https://endgameviable.com/gaming/2022/10/pc-gaming-wasteland/</link>
	  <pubDate>Sun, 16 Oct 2022 10:11:36 -0400</pubDate>
	  <guid>https://endgameviable.com/gaming/2022/10/pc-gaming-wasteland/</guid>
	  <description>&lt;p&gt;I guess this is the inevitable effect of the pandemic.&lt;/p&gt;</description>
    </item>
  </channel>
</rss>`

const secondRSS = `<?xml version="1.0" encoding="utf-8" ?>
<rss version="2.0" xmlns:atom="http://www.w3.org/2005/Atom" xmlns:itunes="http://www.itunes.com/dtds/podcast-1.0.dtd">
  <channel>
    <title>Endgame Viable</title>
    <link>https://endgameviable.com/</link>
    <item>
      <title>Twitter Firestorm, Part 2</title>
      <link>https://endgameviable.com/post/2022/11/twitter-firestorm-part-2/</link>
      <pubDate>Sun, 20 Nov 2022 16:43:30 -0500</pubDate>
      <guid>https://endgameviable.com/post/2022/11/twitter-firestorm-part-2/</guid>
      <description>twitter firestorm body</description>
    </item>
    <item>
      <title>ActivityPub And Me, Part 1 of ?</title>
      <link>https://endgameviable.com/dev/2022/11/activitypub-and-me-part-1/</link>
      <pubDate>Fri, 18 Nov 2022 13:25:34 -0500</pubDate>
      <guid>https://endgameviable.com/dev/2022/11/activitypub-and-me-part-1/</guid>
      <description>&lt;p&gt;&lt;img src=&#34;https://media.endgameviable.com/img/2019/08/html-header-image.jpg&#34; /&gt;&lt;/p&gt;&lt;p&gt;I&amp;rsquo;ve been on a learning rampage on the topic of &lt;a href=&#34;https://en.wikipedia.org/wiki/Fediverse&#34;&gt;the fediverse&lt;/a&gt; lately, and there&amp;rsquo;s plenty of material for writing blog posts.&lt;/p&gt;
&lt;p&gt;With everyone &lt;a href=&#34;https://twitterisgoinggreat.com/#just-setting-up-their-quitters&#34;&gt;freaking out about Twitter again today&lt;/a&gt;, it seems like I good time to publish this draft. However, I&amp;rsquo;m specifically &lt;em&gt;not&lt;/em&gt; going to:&lt;/p&gt;
&lt;p&gt;Anyhoo, those are some of my ActivityPub interests and the obstacles I need to overcome to make it happen.&lt;/p&gt;</description>
    </item>
	<item>
	  <title>PC Gaming Wasteland</title>
	  <link>https://endgameviable.com/gaming/2022/10/pc-gaming-wasteland/</link>
	  <pubDate>Sun, 16 Oct 2022 10:11:36 -0400</pubDate>
	  <guid>https://endgameviable.com/gaming/2022/10/pc-gaming-wasteland/</guid>
	  <description>&lt;p&gt;I guess this is the inevitable effect of the pandemic.&lt;/p&gt;</description>
    </item>
  </channel>
</rss>`

func TestRSSWatcher_ParseItems(t *testing.T) {
	w := FeedWatcher{
		itemParser: gofeedParser{
			parser: gofeed.NewParser(),
		},
		known: make(map[string]time.Time),
	}
	r := bytes.NewBufferString(firstRSS)
	newItems, err := w.parseItems(r)
	require.NoError(t, err)
	assert.NotNil(t, newItems)
	require.Equal(t, 2, len(newItems))
}

type mockNewItem struct {
	mock.Mock
}

func (m *mockNewItem) NewItem(item Item) {
	m.Called(item)
}

func (m *mockNewItem) StatusCode(code int) {
	m.Called(code)
}

func (m *mockNewItem) Error(err error) {
	m.Called(err)
}

func TestRSSWatcher_CheckModified(t *testing.T) {
	srv := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		// Primitive Last-Modified handling
		if r.Header.Get("If-None-Match") == "ABC" && r.Header.Get("If-Modified-Since") == "123" {
			w.WriteHeader(http.StatusNotModified)
			return
		}
		w.Header().Add("ETag", "ABC")
		w.Header().Add("Last-Modified", "123")
		fmt.Fprintf(w, firstRSS)
	}))
	defer srv.Close()

	mockHandler := &mockNewItem{}
	mockHandler.On("StatusCode", 200).Once()
	mockHandler.On("NewItem", mock.Anything).Times(2) // 2 items in the first rss feed
	mockHandler.On("StatusCode", 304).Once()

	w := FeedWatcher{
		URL:     srv.URL,
		Client:  http.Client{},
		Handler: mockHandler,
		itemParser: gofeedParser{
			parser: gofeed.NewParser(),
		},
		known: make(map[string]time.Time),
	}

	assert.NoError(t, w.Check(context.Background()))

	// Second time should get unmodified
	assert.NoError(t, w.Check(context.Background()))

	mockHandler.AssertExpectations(t)
}

func TestRSSWatcher_CheckNewItem(t *testing.T) {
	srv1 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, firstRSS)
	}))
	defer srv1.Close()

	srv2 := httptest.NewServer(http.HandlerFunc(func(w http.ResponseWriter, r *http.Request) {
		fmt.Fprintf(w, secondRSS)
	}))
	defer srv2.Close()

	mockHandler := &mockNewItem{}
	mockHandler.On("StatusCode", 200).Once()
	mockHandler.On("NewItem", mock.Anything).Times(2) // 2 items in the first rss feed
	mockHandler.On("StatusCode", 200).Once()
	mockHandler.On("NewItem", mock.Anything).Once() // only 1 new item in second rss feed

	w := FeedWatcher{
		URL:     srv1.URL,
		Client:  http.Client{},
		Handler: mockHandler,
		itemParser: gofeedParser{
			parser: gofeed.NewParser(),
		},
		known: make(map[string]time.Time),
	}

	assert.NoError(t, w.Check(context.Background()))

	w.URL = srv2.URL

	// Second time should get only 1 new item
	assert.NoError(t, w.Check(context.Background()))

	mockHandler.AssertExpectations(t)
}

func parseFeed(url string) ([]Item, error) {
	w := FeedWatcher{
		itemParser: gofeedParser{
			parser: gofeed.NewParser(),
		},
		known: make(map[string]time.Time),
	}
	resp, err := http.Get(url)
	if err != nil {
		return nil, err
	}
	defer resp.Body.Close()
	return w.parseItems(resp.Body)
}

func TestFeedWatcher_RSSFeed(t *testing.T) {
	// Test RSS parsing doesn't crash
	items, err := parseFeed("https://endgameviable.com/index.xml")
	assert.NoError(t, err)
	assert.True(t, len(items) > 0)

	// CNN is squirley because of its dates
	items, err = parseFeed("http://rss.cnn.com/rss/cnn_topstories.rss")
	assert.NoError(t, err)
	assert.True(t, len(items) > 0)
}

func TestFeedWatcher_AtomFeed(t *testing.T) {
	// Test Atom parsing doesn't crash
	// YouTube feeds are Atom feeds
	items, err := parseFeed("http://www.youtube.com/feeds/videos.xml?channel_id=UCS0bzW6KmGmiq0o2Xs8VwgQ")
	assert.NoError(t, err)
	assert.True(t, len(items) > 0)
}

func TestFeedWatcher_JSONFeed(t *testing.T) {
	// Test JSON parsing doesn't crash
	// NPR has JSON feeds though they aren't easy to find
	items, err := parseFeed("https://www.npr.org/feeds/1019/feed.json")
	assert.NoError(t, err)
	assert.True(t, len(items) > 0)
}
