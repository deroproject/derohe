# Welcome to DERO
[Twitter](https://twitter.com/DeroProject) [Discord](https://discord.gg/H95TJDp) [GitHub](https://github.com/deroproject/derohe) [Explorer](https://testnetexplorer.dero.io) [Wiki](https://wiki.dero.io) [Web Wallet](https://wallet.dero.io/)

DERO is a blockchain with Smart Contracts preserving your privacy through multiple features while staying fast, secure, and accessible to all people easily.

DERO is running since December 2017 and was previously running on a unique implementation of CryptoNote protocol with blockDAG and as migrated to this unique version for Smart Contracts, Services, better performance, better privacy and more security.

## DERO Homomorphic Encryption

### What is "Homomorphic Encryption" ?

Homomorphic encryption (HE) is a form of encryption allowing one to perform calculations on encrypted data without decrypting it first. The result of the computation is in an encrypted form, when decrypted the output is the same as if the operations had been performed on the unencrypted data.

It can be used for privacy-preserving outsourced storage and computation. This allows data to be encrypted and out-sourced to commercial cloud environments for processing, all while encrypted. In highly regulated industries, such as health care, and can also be used to enable new services by removing privacy barriers inhibiting data sharing. For example, predictive analytics in health care can be hard to apply via a third party service provider due to medical data privacy concerns, but if the predictive analytics service provider can operate on encrypted data instead, these privacy concerns are diminished.

For more details of what is Homomorphic Encryption, please see [Wikipedia](https://en.wikipedia.org/wiki/Homomorphic_encryption).  

## Summary

- [Welcome to DERO](#welcome-to-dero)
  - [DERO Homomorphic Encryption](#dero-homomorphic-encryption)
    - [What is "Homomorphic Encryption" ?](#what-is-homomorphic-encryption-)
  - [Summary](#summary)
  - [About DERO Project](#about-dero-project)
      - [Features](#features)
  - [Transactions Sizes](#transactions-sizes)
  - [Network Ports](#network-ports)
    - [Mainnet](#mainnet)
    - [Testnet](#testnet)
  - [Technical](#technical)
  - [DERO blockchain salient features](#dero-blockchain-salient-features)
      - [**Erasure Coded Blocks**](#erasure-coded-blocks)
      - [Client Protocol](#client-protocol)
      - [Proving DERO Transactions](#proving-dero-transactions)
  - [DERO Installation](#dero-installation)
    - [Installation From Source](#installation-from-source)
    - [Installation From Binary](#installation-from-binary)
    - [Running DERO Daemon](#running-dero-daemon)
    - [DERO CLI Wallet](#dero-cli-wallet)
    - [DERO Explorer](#dero-explorer)

## About DERO Project

DERO is running since December 2017 and was previously running on a unique [implementation](https://github.com/deroproject/derosuite) of CryptoNote protocol with blockDAG and as migrated to this unique version for Smart Contracts, Services, better performance, better privacy and more security.

Consensus algorithm is PoW based on [AstroBWT](https://github.com/deroproject/astrobwt), a ASIC/FPGA/GPU resistant CPU mining algorithm to improve decentralization and lower the barrier of joining the network.

DERO is industry leading and the first blockchain to have Homomorphic Encryption, bulletproofs and a fully TLS encrypted network.

The fully distributed ledger processes transactions with a  16s (sixty-seconds) average block time and is secure against majority hashrate attacks.

DERO is the first Homomorphic Encryption based blockchain to have Smart Contracts contracts on its native chain without any extra layers or secondary blockchains.

At present DERO has implemented Smart Contracts for previous mainnet implementation running on CryptoNote protocol which can be found [here](https://github.com/deroproject/documentation/blob/master/testnet/stargate.md).

#### Features

-  **Homomorphic account based model** 
Check blockchain/transaction_execute.go line 82-95

- **Instant account balances**
It need only 66 bytes of data from the blockchain.

- **~~DAG/MINIDAG with 1 miniblock every second~~**
**NOTE**: This as been replaced by a mini block system. Each block has 10 mini blocks to rewards up to 10 differents miners at the same time for each block, by splitting the difficulty and the result of the final block.
 
- **Mining Decentralization**
No more mining pools needed, with a daily ~54000 mini blocks.
Solo Mining is much more available than any others blockchains.

- **Erasure coded blocks**
Worlds first Erasure Coded Propagation protocol, which allows 100x block size without increasing propagation delays, lower bandwidth requirements and provide very low propagation time. 

- **Light weight and efficient wallets**
No more chain scanning or wallet scanning the whole chain to detect funds, no key images. By connecting to a synchronized node, you will retrieve your available funds in few seconds.

- **Small disk cost for blockchain account**
Fixed per account cost of 66 bytes in blockchain only which allow a immense scalability.

- **Anonymous Transactions**
Provide completely anonymous transactions and deniability thanks to Many-Out-Of-Many Proofs using Bulletproofs and Sigma Protocol, nobody except you and the receiver account will know the real parties of the transaction.
 
- **Fixed Transaction Size**
Only ~2.5KB for a transaction using ring size set to 8, or ~3.4 KB with ring size set to 16.
Transactions size have a logarithm growth based on the anonymity set and must be chosen in powers of 2.

- **Homomorphic Assets**
programmable Smart Contracts with fixed overhead per asset.
Your Smart Contract is open source but its data related to balances are completely encrypted like the native coin.

- **Pruning Blockchain**
Allows chain pruning on daemons to control growth of data on daemons and keep a low disk usage. This allows immense scability as you can reduce a blockchain of few hundred GBs to only few GBs while still being secure using merkle proofs.<br>
_Example_: disk requirements of 1 billion accounts (assumming it does not want to keep history of transactions, but keeps proofs to prove that the node is in sync with all other nodes)<br>
Requirement of 1 account is only 66 bytes
Assumming storage overhead per account of 128 bytes (which is constant)
Total requirements = (66 + 128)GB ~ 200GB
Assuming we are off by factor of 4, its only 800GB.<br>
Note that, Even after 1 trillion transactions, 1 billion accounts will consume 800GB only, If history is not maintained, and everything still will be in proved state using merkle roots.
And so, Even Raspberry Pi can host the entire chain.

- **Low Transaction Generation Time**
Generating a transaction takes less than 25 ms.

- **Low Transaction Verification Time**
Transaction verification takes even less than 25ms.

- **No trusted setup, no hidden parameters**
Everything is open-source, available, to anyone to provide trustless and fully decentralized blockchain.
 
- **Provability**
Senders of a transaction can prove to receivers what amount they have send without revealing themselves.


## Transactions Sizes

| Ring Size | Transaction Size (in bytes) |
|:---------:|:---------------------------:|
|     2     |             1553            |
|     4     |             2013            |
|     8     |             2605            |
|     16    |             3461            |
|     32    |             4825            |
|     64    |             7285            |
|    128    |            11839            |
|    512    |            ~35000           |

**NOTE:** Plan to reduce TX sizes further.

## Network Ports

### Mainnet

- **P2P Default Port**: 10101
- **RPC Default Port**: 10102
- **Wallet RPC Default Port**: 10103

### Testnet

- **P2P Default Port**: 40401
- **RPC Default Port**: 40402
- **Wallet RPC Default Port**: 40403

## Technical

For specific details of current DERO core (daemon) implementation and capabilities, see below:

- ~~**DAG**: No orphan blocks, No soft-forks.~~
**NOTE**: This feature has been disabled for mainnet to reduce load for small devices.

- **BulletProofs**: Non Interactive Zero-Knowledge Range-Proofs (NIZK)

- **AstroBWT**: This is memory-bound algorithm. This provides assurance that all miners are equal. ( No miner has any advantage over common miners).

- **P2P Protocol**: This layers controls exchange of blocks, transactions and blockchain itself.

- **Pederson Commitment**: (Part of ring confidential transactions): Pederson commitment algorithm is a cryptographic primitive that allows user to commit to a chosen value while keeping it hidden to others. Pederson commitment is used to hide all amounts without revealing the actual amount. It is a homomorphic commitment scheme.

- **Homomorphic Encryption**: Homomorphic Encryption is used to to do operations such as addition/substraction to settle balances with data being always encrypted (Balances are never decrypted before/during/after operations in any form.).

- **Homomorphic Ring Confidential Transactions**: Gives untraceability, privacy and fungibility while making sure that the system is stable and secure.

- **Core-Consensus Protocol implemented**: Consensus protocol serves 2 major purpose:
	- Protects the system from adversaries and protects it from forking and tampering.
	- Next block in the chain is the one and only correct version of truth (balances).

- **Proof-of-Work(PoW) algorithm**: PoW part of core consensus protocol which is used to cryptographically prove that X amount of work has been done to successfully find a block.

- **Difficulty algorithm**: Difficulty algorithm controls the system so as blocks are found roughly at the same speed, irrespective of the number and amount of mining power deployed.

- **Serialization/De-serialization of blocks**: Capability to encode/decode/process blocks.

- **Serialization/De-serialization of transactions**: Capability to encode/decode/process transactions.

- **Transaction validity and verification**: Any transactions flowing within the DERO network are validated, verified.

- **Socks proxy**: Socks proxy has been implemented and integrated within the daemon to decrease user identifiability and improve user anonymity.

- **Interactive daemon**: can print blocks, txs, even entire blockchain from within the daemon
	-	`version`, `peer_list` `status`, `diff`, `print_bc`, `print_block`, `print_tx` and several other commands implemented

- **Networks**: DERO Daemon has both mainnet and testnet support.

- **Enhanced Reliability, Privacy, Security, Useability, Portabilty assured.**

## DERO blockchain salient features

- 16 Second Block time.
- Extremely fast transactions with one minute/block confirmation time.
- SSL/TLS P2P Network.
- Homomorphic: Fully Encrypted Blockchain
- Ring signatures.
- Fully Auditable Supply.
- DERO blockchain is written from scratch in Golang.
- Developed and maintained by original developers.

#### **Erasure Coded Blocks**

Traditional Blockchains process blocks as single unit of computation(if a double-spend tx occurs within the block, entire block is rejected). As soon as a block is found, it is sent to all its peers.DERO blockchain erasure codes the block into 48 chunks, dispersing and chunks are dispersed to peers randomly.Any peer receiving any 16 chunks( from 48 chunks) can regerate the block and thus lower overheads and lower propagation time.

#### Client Protocol

Traditional Blockchains process blocks as single unit of computation(if a double-spend tx occurs within the block, entire block is rejected). However DERO network accepts such blocks since DERO blockchain considers transaction as a single unit of computation.DERO blocks may contain duplicate or double-spend transactions which are filtered by client protocol and ignored by the network. DERO DAG processes transactions atomically one transaction at a time.

#### Proving DERO Transactions

DERO blockchain is completely private, so anyone cannot view, confirm, verify any other's wallet balance or any transactions.

So to prove any transaction you require *TXID* and *deroproof*.
deroproof can be obtained using `get_tx_key` command in dero-wallet-cli.

Enter the *TXID* and *deroproof* in [DERO Explorer](https://testnetexplorer.dero.io)
![DERO Explorer Proving Transaction](https://github.com/deroproject/documentation/raw/master/images/explorer-prove-tx.png)

## DERO Installation

DERO is written in golang and very easy to install both from source and binary.
  
### Installation From Source

First you need to install Golang if not already, minimum version required for Golang is 1.17.

In go workspace, execute:
`go get -u github.com/deroproject/derohe/...`

When the command has finished, check go workspace bin folder for binaries.
For example, on Linux machine the following binaries will be created:
-  `derod-linux-amd64`: DERO Daemon
-  `dero-wallet-cli-linux-amd64`: DERO CLI Wallet
-  `explorer-linux-amd64`: DERO Explorer (Yes, DERO has prebuilt personal explorer also for advance privacy users)

### Installation From Binary

Download [DERO binaries](https://github.com/deroproject/derohe/releases)  for ARM, INTEL, MAC platform and Windows, Mac, FreeBSD, OpenBSD, Linux (or any others availables platforms) operating systems.

### Running DERO Daemon

Run derod.exe or derod-linux-amd64 depending on your operating system. It will start syncing.

- DERO daemon core cryptography is highly optimized and fast.
- Use dedicated machine and SSD for best results.
- VPS with 2-4 Cores, 4GB RAM,15GB disk is recommended.

![DERO Daemon](https://raw.githubusercontent.com/deroproject/documentation/master/images/derod1.png)

### DERO CLI Wallet
DERO cmdline wallet is menu based and very easy to operate.

Use various options to create, recover, transfer balance etc.

**NOTE:** DERO cmdline wallet by default connects DERO daemon running on local machine on port 20206.

If DERO daemon is not running start DERO wallet with --remote option like following:

**./dero-wallet-cli-linux-amd64 --remote**

![DERO Wallet](https://raw.githubusercontent.com/deroproject/documentation/master/images/wallet-recover2.png)

### DERO Explorer

[DERO Explorer](https://explorer.dero.io/) is used to check and confirm transaction on DERO Network.

DERO users can run their own explorer on local machine and can [browse](http://127.0.0.1:8080) on local machine port 8080.

![DERO Explorer](https://github.com/deroproject/documentation/raw/master/images/dero_explorer.png)