1] ### DEROHE Installation, https://github.com/deroproject/derohe  

        DERO is written in golang and very easy to install both from source and binary.
Installation From Source:  
    Install Golang, minimum Golang 1.17 required.
    In go workspace: go get -u github.com/deroproject/derohe/...
    Check go workspace bin folder for binaries.
    For example on Linux machine following binaries will be created:
        derod-linux-amd64 -> DERO daemon.
        dero-wallet-cli-linux-amd64 -> DERO cmdline wallet.
        explorer-linux-amd64 -> DERO Explorer. Yes, DERO has prebuilt personal explorer also for advance privacy users.

Installation From Binary  
        Download DERO binaries for ARM, INTEL, MAC platform and Windows, Mac, FreeBSD, OpenBSD, Linux etc. operating systems.  
https://github.com/deroproject/derohe/releases

2] ### Running DERO Daemon  
./derod-linux-amd64 

3] ### Running DERO Wallet (Use local or remote daemon) 
./dero-wallet-cli-linux-amd64 --remote  
https://wallet.dero.io [Web wallet]

4] ### DERO Mining Quickstart
Run miner with wallet address and no. of threads based on your CPU.  
./dero-miner --mining-threads 2 --daemon-rpc-address=http://testnetexplorer.dero.io:40402 --wallet-address deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p  

NOTE: Miners keep your system clock sync with NTP etc.  
Eg on linux machine: ntpdate pool.ntp.org 
For details visit http://wiki.dero.io
