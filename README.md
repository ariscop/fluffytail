fluffytail
==========

stream your system logs to IRC!

fluffytail is a simple log streamer that pipes the output of `journalctl -f -l`
to a specified IRC channel. It is seriously drop-dead simple. Simply copy
`fluffytail.conf.example` to `fluffytail.conf` (or anywhere else you want,
specified by the `-conf` option), modify it appropriately, and run! The options
should be self-explanatory or commented in the example file.

The configuration file format looks something like:

```ini
; fluffytail config
[irc]
host = irc.example.net:6697
password = passwords_are_magic
usessl = on
channel = "#serverlogs"

[bot]
nick = adagiodazzle
user = fluffy
senddelay = 300 # milliseconds
onconnect = OPER adagio opensesame
onconnect = PRIVMSG NickServ :IDENTIFY adagiodazzle opensesame
```
