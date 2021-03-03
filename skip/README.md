# skip

**skip** (SCION kludge in *p*rowsers, also "ship" in many languages and so fitting
with the lighthouse/beacon scheme :boat:) is a poor man's browser integration
for SCION.

skip uses a [Proxy auto-config](https://en.wikipedia.org/wiki/Proxy_auto-config)
file to forward all requests with a SCION destination to a proxy server running
as a (native) binary on localhost.
As this mechanism does not let us support a separate protocol identifier nor allow
looking up whether a name refers to a SCION address, we identify SCION addresses
as either:
  * the host name of a SCION host with an appended  pseudo-TLD `.scion`, e.g.
    `http://www.scionlab.org.scion`, or
  * a mangled SCION address in the form `<ISD>-<AS id with
    underscores>-<host>`, e.g. `http://17-ffaa_0_1101-129.132.121.164/`

## Installation

* Build the `scion-skip` binary by running `make scion-skip` (see
  [Build](../README.md#build) in the main README).

* Install the `skip.pac` as an "Automatic proxy configuration".

  In Firefox (currently v84.0), navigate to
  **Preferences** / **General** / **Network Settings**, enable "Automatic proxy
  configuration URL" and enter `http://localhost:8888/skip.pac`.
  Adapt the address if you're running skip on a non-default address with `--bind`.

## Usage

This requires a running SCION endhost stack, i.e. a running SCION dispatcher
and SCION daemon.  Please refer to '[Running](../../README.md#Running)' in this
repository's main README and the [SCIONLab tutorials](https://docs.scionlab.org) to get started.

Start `bin/scion-skip` and keep it running in the background.

Enter SCION addresses in the URL bar of your browser, mangled as described above:
  * [http://www.scionlab.org.scion](http://www.scionlab.org.scion)
  * [http://17-ffaa_0_1101-129.132.121.164/](http://17-ffaa_0_1101-129.132.121.164/)

## Limitations

* Does not support HTTPS
* Does not support WebSockets (HTTP CONNECT) method
* Does not allow specifiying the protocol (e.g. as
  `scion+http://www.scionlab.org`) but instead uses a kludgy pseudo-TLD to
  identify SCION hosts.

Obviously this is not great, but hey, it's a start. Some inspiration for how to
to build something more advanced can be found in this extensions for the gopher
protocol, [OverbiteNX](https://github.com/classilla/overbitenx).
