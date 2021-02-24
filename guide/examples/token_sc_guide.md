## Dero Stargate DVM Smart Contracts guide to install and test various function of private token Smart Contract.


**All wallet Addressess need to be registerd first with SC before they need to interact with. This condition will be removed in future.**  


**Download** Dero Stargate testnet [source](https://git.dero.io/DeroProject/derosuite_stargate) and [binaries](https://git.dero.io/DeroProject/Dero_Stargate_testnet_binaries).

**Start Dero daemon in testnet mode.**
```
./derod-linux-amd64  --testnet
```

**Start Dero wallet in testnet.** 
```
dero-wallet-cli-linux-amd64 --rpc-server --wallet-file testnet.wallet --testnet
```

**Start Dero wallet second instance to test transfer/ownership functions etc.**
```
dero-wallet-cli-linux-amd64 --wallet-file testnet2.wallet --testnet --rpc-server --rpc-bind=127.0.0.1:40403
```

**Dero testnet Explorer, Not required but if you want to host your own explorer for privacy reasons.**
```
./explorer-linux-amd64  --http-address=0.0.0.0:8080                  
```
Connect to explorer using browser on localhost:8080


**Dero Stargate Testnet Explorer**  
[https://testnetexplorer.dero.io/ ](https://testnetexplorer.dero.io/)


**To send DERO to multiple users in one transaction**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":100000,"destination":"deto1ADDRESS1"},{"amount":300000,"destination":"deto1ADDRESS12}] }}' -H 'Content-Type: application/json'
```

**DERO has 2 types of SCs, public and private.**
1. Public SCs are public with all data/code/exchanges are public.
1. Private SCs have their smart contract data public. But no one knows how many tokens a particular user own or how much is he sending or how much is he receiving. Best example is to understand private SCs as banks and private tokens as cash. Once cash is out from the bank, Bank doesn't know "who has what amount or how is it being used/sent/received etc.". This makes all private tokens completely private.

**Installing Private Smart Contract.**  
 [Download token.bas](https://git.dero.io/DeroProject/derosuite_stargate/src/master/cmd/dvm/token.bas)
```
curl  --request POST --data-binary   @token.bas http://127.0.0.1:40403/install_sc
```

**To check private token balance in wallet, type this command in wallet.**
```
balance SCID
```

**Download SC Code,check SC balance and variables from chain**
```
curl http://127.0.0.1:40402/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getsc","params":{ "scid":"YourSCID" , "code":true}}' -H 'Content-Type: application/json'
```


**Examples of various private token Smart Contract functions**
**Eg: To send private tokens from one wallet to another wallet, this does not involve SC**
**Eg: this also showcases to send multiple assets( DERO and other tokens on DERO Network) within a single transaction**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":1,"destination":"DEROReceiverWalletAddress"},{"amount":1,"destination":"TokenReceiverWalletAddress","scid": "SCIDofToken" }] }}' -H 'Content-Type: application/json'
```


**Eg: Convert DERO to tokens 1:1 swap, we are swapping 44 DERO atomic units(DERI) to get some tokens**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{"transfers":[{"amount":1,"destination":"detoAnyRandomAddressFromExplorer", "burn":44}],"scid":"aacaa7bb2388d06e523e5bc0783e4e131738270641406c12978155ba033373af", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"IssueTOKENX"}] }}' -H 'Content-Type: application/json'
```


**Convert tokens to DERO 1:1 swap, we are swapping 9 token atomic units to get 9 DERO atomic units**
**This tx shows transferring tokens natively, no dero fees etc, this is under evaluation,**  
**Currently these show as coinbase rewards **
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{"transfers":[{"scid":"SCID", "amount":1,"destination":"detoAnyRandomAddressFromExplorer", "burn":9}],"scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"ConvertTOKENX"}] }}' -H 'Content-Type: application/json'
```

**Eg: To withdraw balance only for smart contract owner**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":1,"destination":"detoAnyRandomAddressFromExplorer"}],"scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"Withdraw"}, {"name":"amount","datatype":"U","value":2 }] }}' -H 'Content-Type: application/json'
```

**Eg: To transfer ownership of smart contract to new address/owner**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":1,"destination":"detoAnyRandomAddressFromExplorer"}],"scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"TransferOwnership"}, {"name":"newowner","datatype":"S","value":"detoAddressForOwnershipReceiver" }] }}' -H 'Content-Type: application/json'

```

**Eg: To claim ownership of smart contract**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":1,"destination":"detoAnyRandomAddressFromExplorer"}],"scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"ClaimOwnership"}] }}' -H 'Content-Type: application/json'
```


**Eg: To update smart contract code**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":1,"destination":"detoAnyRandomAddressFromExplorer"}],"scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"UpdateCode"}, {"name":"code","datatype":"S","value":"new code should be placed here" }] }}' -H 'Content-Type: application/json'


```
