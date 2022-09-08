# Hello DRKey

An example application that shows how to use DRKey to derive or obtain a key.

## Walkthrough:

The application mimics the behavior using DRKey that would be observed by both a client and a server.
The client uses the slow path to obtain a DRKey, and the server uses the fast path.

Slow path (client):
1. Obtain a connection to the designated Control Service and encapsulate it in the `Client` class.
1. Obtain the metadata for the DRKey.
1. Request the DRKey with that metadata.

Fast path (server):
1. Obtain a connection to the designated Control Service and encapsulate it in the `Server` class.
1. Obtain the Secret Value for that metadata. The Secret Value does not change with the destination host.
1. Derive the DRKey with the SecretValue and the metadata.

Both slow and fast paths should obtain the same key.
And both slow and fast path are measured for performance and their times displayed at the end.

## CS Configuration

For this example to work, you must configure your devel SCION with an appropriated `gen` directory.
Follow these steps:

1. Create a local topology with the `tiny.topo` description: `./scion.sh topology -c ./topology/tiny.topo`.
1. Add the following entry to `gen/ASff00_0_111/cs1-ff00_0_111-1.toml`:
```
[drkey.delegation]
scmp = [ "<application network address>",] (e.g., scmp = [ "127.0.0.1",] )
```
1. Restart scion with `./scion.sh stop; ./scion.sh start nobuild` and run hellodrkey.
