# viadown
Download mirroring tool

Suppose you are running the same Linux distribution on a number of computers.
Every time an update is run, each of these machines will try and download
packages from distribution mirrors. To save bandwidth and time, you could setup
a local mirror, but why download all of packages instead of just the few that
are needed? You could setup a caching proxy, but what about those package
managers that do not play well with proxies?

`viadown` is a proxying/mirroring tool that downloads requested packages to its
local cache while serving the same data to whoever requested it.

## Building

```
go get -v github.com/bboozzoo/viadown
viadown
```

## Configuration

All hardcoded now. `viadown` will listen on `0.0.0.0:8080`. Downloads land in
`./tmp`.
