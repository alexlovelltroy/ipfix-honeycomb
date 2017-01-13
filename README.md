# IPFIX-HONEYCOMB

This is a very small utility that I use to stream netflow data from my openbsd router to [honeycomb.io](honeycomb.io).

To use it for yourself, you'll need a honeycomb account and an openbsd router configured to stream netflow data.

## Set Up Netflow

Set up your netflow device by creating the hostname.pflow0 file with contents like this:

```
flowsrc <internal router ip address> flowdst <internal router ip address>:9995
pflowproto 10
```
You can set the flow destination `flowdst` to a remote machine, but I do everything on my router.

## Add a configuration file

We use an `app.conf` toml-based file for the honeycomb writekey and dataset.

```
WriteKey = ##########
Dataset = ###########
```

## Stream data
I use [socat](http://www.dest-unreach.org/socat/) to manage the udp stream even though I could certainly add that code.

`socat udp-recv:9995 - |./ipfix_honeycomb`