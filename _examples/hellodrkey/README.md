# Hello DRKey

An example application that shows how to use DRKey to derive or obtain a key.

## Walkthrough:

The application mimics the behavior using DRKey that would be observed by both a client and a server.
The client uses the slow path to obtain a DRKey, and the server uses the fast path.

Slow path (client):
1. Obtain a connection to the designated `sciondForClient` and encapsulate it in the `Client` class.
1. Obtain the metadata for the DRKey.
1. Request the DRKey with that metadata.

Fast path (server):
1. Obtain a connection to the designated `sciondForServer` and encapsulate it in the `Server` class.
1. Obtain the delegation secret for that metadata. The delegation secret does not change with the destination host.
   You can check that in `dsForServer`. The delegation secret is stored.
1. Derive the DRKey with the delegation secret and the metadata.

Both slow and fast paths should obtain the same key.
And both slow and fast path are measured for performance and their times displayed at the end.

## CS Configuration

For this example to work, you must configure your devel SCION with an appropriated `gen` directory.
Follow these steps:

1. Create a local topology with the `tiny.topo` description: `./scion.sh topology -c ./topology/tiny.topo`.
1. Restart scion with `./scion.sh stop; ./scion.sh start nobuild`.
1. Run hellodrkey.
