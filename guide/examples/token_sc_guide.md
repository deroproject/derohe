## Dero Stargate DVM Smart Contracts guide to install and test various function of private token Smart Contract.

**Notes:**  
1] All wallet Addressess need to be registerd first with SC before they need to interact with. This condition will be removed in future.  


**Download** Dero Stargate testnet [source](https://github.com/deroproject/derohe) and [binaries](https://github.com/deroproject/derohe/releases).

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


**Examples of various private Smart Contract Token functions**  
**To send private tokens from one wallet to another wallet, this does not involve SC**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":100000,"destination":"deto1qxsqqnk4zargp7hyr7euk29mxkwfna9999mpylh3hy2zp9xkg5hmcvg4xagvj","scid":"69e3168d69630d54f6ee93e06fc954d7e31cb28fb5bf77a9e4ee2e2928e66c40"}] }}' -H 'Content-Type: application/json'
```  
**NOTE:**  Destination/Receiver wallet should be registered first to that SC before receiving any transactions related to this SC. This pre-registration requirement of any wallet with SC will be removed in future. To register any wallet with SC make any deposit to that SC once.  




**Convert DERO to Tokens**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{"sc_dero_deposit":200000,"scid":"69e3168d69630d54f6ee93e06fc954d7e31cb28fb5bf77a9e4ee2e2928e66c40", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"IssueTOKENX"}] }}' -H 'Content-Type: application/json'
```  
**NOTE:**  In [above SC](https://testnetexplorer.dero.io/tx/69e3168d69630d54f6ee93e06fc954d7e31cb28fb5bf77a9e4ee2e2928e66c40) 2 DERO is swapped to 2 TOKENX. For swap ratio look into Smart Contract code.  




**Convert Tokens to DERO**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{"sc_token_deposit":200000,"scid":"69e3168d69630d54f6ee93e06fc954d7e31cb28fb5bf77a9e4ee2e2928e66c40", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"ConvertTOKENX"}] }}' -H 'Content-Type: application/json'
```  
**NOTE:**  In [above SC](https://testnetexplorer.dero.io/tx/69e3168d69630d54f6ee93e06fc954d7e31cb28fb5bf77a9e4ee2e2928e66c40) 2 TOKENX is swapped to 2 DERO. For swap ratio look into Smart Contract code.   
Currently these show as coinbase rewards.  




**Eg: To withdraw DERO balance from Smart Contract**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{ "scid":"69e3168d69630d54f6ee93e06fc954d7e31cb28fb5bf77a9e4ee2e2928e66c40", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"Withdraw"}, {"name":"amount","datatype":"U","value":100000 }] }}' -H 'Content-Type: application/json'
```  
**NOTE:**  From [above SC](https://testnetexplorer.dero.io/tx/69e3168d69630d54f6ee93e06fc954d7e31cb28fb5bf77a9e4ee2e2928e66c40) 1 DERO will be transferred from SC to wallet. Only owner of Smart Contract can initate the above command. SC must have that balance.  





**Eg: To transfer ownership of smart contract to new address/owner**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{ "scid":"69e3168d69630d54f6ee93e06fc954d7e31cb28fb5bf77a9e4ee2e2928e66c40", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"TransferOwnership"}, {"name":"newowner","datatype":"S","value":"detoAddressForOwnershipReceiver" }] }}' -H 'Content-Type: application/json'
```  




**Eg: To claim ownership of smart contract**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{ "scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"ClaimOwnership"}] }}' -H 'Content-Type: application/json'
```    





**Eg: To update smart contract code**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{ "scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"UpdateCode"}, {"name":"code","datatype":"S","value":"new code should be placed here" }] }}' -H 'Content-Type: application/json'
```   
**NOTE:**  Please use this command carefully. Try this command several times on testnet before issuing on maiinet to update SC code.  

