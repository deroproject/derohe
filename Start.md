# DERO Installation

Official source code is available here: https://github.com/deroproject/derohe

DERO is written in golang and is very easy to install both from source and binary.

## From sources

First you need to install Golang if not already, minimum version required for Golang is 1.17.

In go workspace, execute:
`go get -u github.com/deroproject/derohe/...`

When the command has finished, check go workspace bin folder for binaries.
For example, on Linux machine the following binaries will be created:
- `derod-linux-amd64`: DERO Daemon
- `dero-wallet-cli-linux-amd64`: DERO CLI Wallet
-  `explorer-linux-amd64`: DERO Explorer  (Yes, DERO has prebuilt personal explorer also for advance privacy users)

## From Binary

Download DERO binaries for ARM, INTEL, MAC platform and Windows, Mac, FreeBSD, OpenBSD, Linux (or any others availables platforms) operating systems.

Official link for releases is https://github.com/deroproject/derohe/releases

## Running DERO Daemon

In a terminal, execute the following command:
`./derod-linux-amd64`

In case you don't want to synchronize the whole DERO Blockchain and want to synchronize really fast, add `--fastsync` option on above command. This option will downloaded a pruned (bootstrapped) chain and will works like any traditional node without using too much disk space.

You can  also use `--help` option to see all available options.
  
## Running DERO Wallet

DERO has a CLI (Command Line Interface) secure and really easy to use !

If you don't want to setup and synchronize a node, execute following command:
`./dero-wallet-cli-linux-amd64 --remote`

If you have a local daemon running, delete `--remote` option.

Official web wallet is at https://wallet.dero.io but its support has been dropped and we strongly recommend to use CLI wallet. 

### DERO Mining Quickstart

Run miner with wallet address and no. of threads based on your CPU.

./dero-miner --mining-threads 2 --daemon-rpc-address=http://testnetexplorer.dero.io:40402 --wallet-address deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p

NOTE: Machine where the daemon is running should keep its system clock sync with NTP for better results in performance.

On a linux machine, run the following command to sync your system clock:
`ntpdate pool.ntp.org`

For more details, please visit http://wiki.dero.io