# DeDTN: Decentralized Digital Twin Network

## Overview

This project proposes a decentralized architecture for managing large-scale Digital Twin Networks (DTNs).

A Digital Twin reproduces real-world environments such as intersections or cities in virtual space.  
By connecting multiple digital twins, it becomes possible to construct large-scale smart city environments.

However, large-scale DTNs face several challenges:

- centralized processing overload
- management across multiple organizations
- sensor/device failures
- malicious attacks
- operational cost and incentives

To address these issues, this project combines:

- off-chain PBFT consensus
- semantic verification
- external PoA blockchain integration

The system performs distributed consensus on semantic data before recording results onto a blockchain.

---

## System Flow
- In the each edge device,
1. Semantic data is generated using AI
2. Watcher detects updates
3. Primary node proposes semantic data for each replica nodes
4. PBFT consensus is executed and Replica nodes verify semantic consistency
5. Consensus result is committed
6. Final result is recorded on a PoA blockchain

---

## Features

- Decentralized Digital Twin management
- PBFT-based fault tolerance
- Semantic-aware validation
- External blockchain recording
- Distributed edge-oriented architecture

---

## Environment

- JetBot / Jetson Nano
- Python
- Geth (PoA blockchain)
- Omniverse
