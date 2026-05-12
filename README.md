# Layered Decentralized Digital Twin Network
## System architecture

- The proposed DeDTN architecture combines off-chain PBFT consensus, semantic synchronization, and external PoA blockchain recording.
<img width="507" height="506" alt="image" src="https://github.com/user-attachments/assets/8c449bab-e049-4207-83ec-dcad21731e00" />


## Experiental Result
https://github.com/user-attachments/assets/2699858b-b92d-4bf1-a804-34d881a5e3b1

<img width="807" height="327" alt="スクリーンショット 2026-05-12 160249" src="https://github.com/user-attachments/assets/083d83dd-0b89-4d0b-b3b3-e5e42770514f" />


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

Layer 2:
- off-chain PBFT consensus 
- semantic verification

Layer 1:
- external PoA blockchain integration

The system performs distributed consensus on semantic data before recording results onto a blockchain.

---

## System Flow
- In the each edge device,
1. Semantic data is generated using AI
2. Watcher detects updates
3. Primary node proposes semantic data to each replica nodes
4. Replica nodes verify semantic consistency and execute PBFT consensus
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

---

# DeDTN Setup Guide


## System Architecture

The system consists of:

* Windows desktop node (Primary / Omniverse)
* Multiple JetBot replica nodes
* Private PoA blockchain network
* Semantic data synchronization

---

## 1. PoA Blockchain Configuration

### Genesis Block Example

```json
{
  "config": {
    "chainId": 12345,
    "homesteadBlock": 0,
    "eip150Block": 0,
    "eip155Block": 0,
    "eip158Block": 0,
    "byzantiumBlock": 0,
    "constantinopleBlock": 0,
    "petersburgBlock": 0,
    "istanbulBlock": 0,
    "clique": {
      "period": 5,
      "epoch": 30000
    }
  },
  "difficulty": "1",
  "gasLimit": "0x47b760"
}
```

### Clique Parameters

```text
period: 5
```

A signer generates a block every 5 seconds.

```text
epoch: 30000
```

Checkpoint blocks are inserted every 30000 blocks.

---

## 2. Initialize the Private Chain

Example:

```bash
geth init genesis.json --datadir ./chain_data
```

---

## 3. Start Geth PoA Nodes

### Example Command

```bash
geth \
  --datadir ./chain_data \
  --networkid 12345 \
  --port 30303 \
  --http \
  --http.addr 0.0.0.0 \
  --http.port 8545 \
  --http.api eth,net,web3,admin,clique,personal,txpool \
  --allow-insecure-unlock \
  --unlock <SIGNER_ADDRESS> \
  --mine \
  --miner.etherbase <SIGNER_ADDRESS> \
  --bootnodes <BOOTNODE_ENODE> \
  console
```

Run this command on each node.

---

## 4. Start Blockchain Read Server

### Windows

```bash
py -m uvicorn semantic_reader2:app --host 0.0.0.0 --port 8000
```

### Attach Geth Console

```bash
geth attach http://127.0.0.1:8545
```

---

## 5. Start Omniverse

1. Launch Omniverse 4.5
2. Load the experimental environment
3. Start semantic synchronization

---

## 6. Start PBFT Node (Windows Primary Node)

### Activate Python Environment

```bash
cd windowsPBFT
.venv\Scripts\activate
```

### Start Semantic Writer

```bash
pythonw windows_semantic_writer.py
```

### Start PBFT Node

```bash
go run main.go config/D.json
```

---

## 7. Start Replica Nodes

### Connect via SSH

```bash
ssh_connect.bat
```

### Run Each Node

Open multiple terminal windows and execute:

```bash
run_all.bat
```

---

## 8. Shutdown

### Stop Python Processes

```powershell
Stop-Process -Name pythonw
```

### Check Running Processes

```bash
ps
```

### Kill Process

```bash
kill <PID>
```

---

## Notes

* Replace all addresses and IPs with your own environment settings.
* Store sensitive information in `.env` files.
* Do not upload private keys to GitHub.
* JetBot / Jetson Nano environment is recommended.

---

## Future Work

* Incentive mechanisms
* Large-scale DTN evaluation
* Smart contract integration
