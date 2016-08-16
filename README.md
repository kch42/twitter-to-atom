twitter-to-atom
===============

Generate an Atom-Feed from a Twitter account.

Installation / Building
-----------------------

You need to have the Go programming language installed. After that, installing is as simple as

	go get github.com/silvasur/twitter-to-atom

Usage
-----

	twitter-to-atom user

This generates an Atom feed on stdout with the recent posts and retweets of the Twitter user `@user`.
