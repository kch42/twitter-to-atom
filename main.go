package main

import (
	"code.google.com/p/go-html-transform/h5"
	"code.google.com/p/go-html-transform/html/transform"
	"code.google.com/p/go.net/html"
	"code.google.com/p/go.tools/blog/atom"
	"encoding/xml"
	"fmt"
	"io"
	"net/http"
	"os"
	"strconv"
	"time"
)

func getattr(attrs []html.Attribute, name string) (val string, found bool) {
	for _, a := range attrs {
		if a.Key == name {
			val = a.Val
			found = true
			return
		}
	}

	return
}

func Textify(node *html.Node) string {
	switch node.Type {
	case html.TextNode:
		return node.Data
	case html.ElementNode:
		for _, att := range node.Attr {
			if att.Key == "alt" {
				return att.Val
			}
		}

		fallthrough
	case html.DocumentNode:
		text := ""
		for n := node.FirstChild; n != nil; n = n.NextSibling {
			text += Textify(n)
		}
		return text
	default:
		return ""
	}
}

type Tweet struct {
	Content string
	From    string
	ID      string
	Date    time.Time
}

func ScrapeTweets(r io.Reader) ([]Tweet, error) {
	t, err := transform.NewFromReader(r)
	if err != nil {
		return nil, fmt.Errorf("Could not scrape profile: %s", err)
	}

	tweets := make([]Tweet, 0)

	t.Apply(func(node *html.Node) {
		var tweet Tweet

		tweet.From, _ = getattr(node.Attr, "data-screen-name")
		tweet.ID, _ = getattr(node.Attr, "data-item-id")

		time_ok := false
		tree := h5.NewTree(node)
		t2 := transform.New(&tree)
		t2.Apply(func(node *html.Node) {
			if ts, ok := getattr(node.Attr, "data-time"); ok {
				if ts_int, err := strconv.ParseInt(ts, 10, 64); err == nil {
					tweet.Date = time.Unix(ts_int, 0)
					time_ok = true
				}
			}
		}, "a.ProfileTweet-timestamp span")
		if !time_ok {
			return
		}

		t2.Apply(func(node *html.Node) {
			tweet.Content = Textify(node)
		}, ".ProfileTweet-text")

		tweets = append(tweets, tweet)

	}, "div.GridTimeline .ProfileTweet")

	return tweets, nil
}

const titlelimit = 80

func (t Tweet) Atomify() *atom.Entry {
	entry := new(atom.Entry)

	entry.Title = "@" + t.From + ": " + t.Content
	if len(entry.Title) > titlelimit {
		entry.Title = string([]rune(entry.Title)[:titlelimit-2]) + " …"
	}

	url := "https://twitter.com/" + t.From + "/status/" + t.ID
	entry.ID = url
	entry.Link = []atom.Link{atom.Link{
		Rel:  "alternate",
		Href: url,
	}}
	entry.Summary = &atom.Text{Type: "text", Body: t.Content}
	entry.Content = &atom.Text{Type: "text", Body: t.Content}
	entry.Author = &atom.Person{
		Name: "@" + t.From,
		URI:  "https://twitter.com/" + t.From,
	}
	entry.Published = atom.Time(t.Date)
	entry.Updated = atom.Time(t.Date)

	return entry
}

func main() {
	os.Exit(Main())
}

func Main() int {
	if len(os.Args) < 2 {
		fmt.Fprintln(os.Stderr, "Need one argument (twitter user name, without the '@')")
		return 1
	}

	user := os.Args[1]

	resp, err := http.Get("https://twitter.com/" + user)
	if err != nil {
		fmt.Fprintf(os.Stderr, "Couldn't download @%s's stream: %s\n", user, err)
		return 1
	}
	defer resp.Body.Close()

	if resp.StatusCode != http.StatusOK {
		fmt.Fprintf(os.Stderr, "Couldn't download @%s's stream: HTTP Status %d %s\n", user, resp.StatusCode, resp.Status)
		return 1
	}

	tweets, err := ScrapeTweets(resp.Body)
	if err != nil {
		fmt.Fprintln(os.Stderr, err)
	}

	feed := atom.Feed{
		Title: "Tweets from @" + user,
		ID:    "https://twitter.com/" + user,
		Link: []atom.Link{
			atom.Link{
				Rel:  "alternate",
				Href: "http://twitter.com/" + user,
			},
		},
		Author: &atom.Person{
			Name: "@" + user,
			URI:  "https://twitter.com/" + user,
		},
	}

	var latest time.Time
	for _, tweet := range tweets {
		feed.Entry = append(feed.Entry, tweet.Atomify())
		if tweet.Date.After(latest) {
			latest = tweet.Date
		}
	}

	feed.Updated = atom.Time(latest)

	enc := xml.NewEncoder(os.Stdout)
	if err := enc.Encode(feed); err != nil {
		fmt.Fprintf(os.Stderr, "Could not encode feed: %s\n", err)
		return 1
	}
	return 0
}
