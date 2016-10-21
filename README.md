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

If you wish to deploy `viadown` to a NAS or a similar always-on device but
running a non-x86 CPU, you will need to cross compile. For instance, to build
`viadown` for BeagleBone run:

```
GOARCH=arm GOARM=7 go build -v
```

## Configuration

Configuration is passed through command line arguments. See `-help` for details.

```
Usage of viadown:
  -cache-root string
        Cache directory path (default "./tmp")
  -client-timeout duration
        Forward request timeout (default 15s)
  -debug
        Enable debug logging
  -listen string
        Listen address (default ":8080")
  -mirrors string
        Mirror list file
```

## Mirror list

Mirror list is a plain text file with a mirror address in every line. Empty
lines, or lines starting with `#` are skipped.

## Example

Assume that I have an ArchLinux installation and `viadown` is deployed to a NAS,
reachable at address 192.168.1.10, port 9999.

Add the following entry to `/etc/pacman.d/mirrorlist`

```
# NAS
Server = http://192.168.1.10:9999/$repo/os/$arch
...
# original entries
Server = http://mirror.js-webcoding.de/pub/archlinux/$repo/os/$arch
Server = http://mirror.de.leaseweb.net/archlinux/$repo/os/$arch
Server = http://archlinux.my-universe.com/$repo/os/$arch

```

Then your `mirrorlist` file for `viadown` should look like this:

```
http://mirror.de.leaseweb.net/archlinux/
http://mirror.js-webcoding.de/pub/archlinux/
http://archlinux.my-universe.com/

```
