# DStore

This repository contains the source code for dstore, a distributed content-addressable file store.

This project is being build as a learning exercise to develop greater understanding of Content Addressable Storage. The main topics being targeted for learning are:

- Distributed systems.
- Peer-to-Peer networking.
- Streaming network I/O.
- File encryption.
- Consensus.

### Usage

--- TODO ---

### Features

- File replication to peers.
- Streaming of arbitrarily large files.
- Multiplexed connections for streaming multiple files concurrently.
- File encryption using CTR mode.


### Ongoing Work/Planned Improvements

- Add HMAC authentication to file encryption.
- Add Viper for clean CLI usage.
- Use mTLS for in-flight encryption.
- Implement file metadata and indexing.
- Implement cryptographic file ownership.
- Collect and gossip peer and network level metadata.
- Research adding consensus layer.