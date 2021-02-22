## Dero Stargate DVM Smart Contracts guide to install and test various function of lottery Smart Contract.

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

**Dero testnet Explorer**
```
./explorer-linux-amd64  --rpc-server-address 127.0.0.1:30306 --http-address=0.0.0.0:8080
```

**Dero Stargate Testnet Explorer**  
[https://testnetexplorer.dero.io/ ](https://testnetexplorer.dero.io/)


**Installing Smart Contract.**  
 [Download lottery.bas](https://git.dero.io/DeroProject/derosuite_stargate/src/master/cmd/dvm/lottery.bas)
```
curl  --request POST --data-binary   @lottery.bas http://127.0.0.1:40403/install_sc
```


** Download SC Code,check balance and variables from chain**
```curl http://127.0.0.1:40402/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getsc","params":{ "scid":"aacaa7bb2388d06e523e5bc0783e4e131738270641406c12978155ba033373af" , "code":true}}' -H 'Content-Type: application/json'


curl http://127.0.0.1:40402/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getsc","params":{ "scid":"aacaa7bb2388d06e523e5bc0783e4e131738270641406c12978155ba033373af" , "code":false, "keysstring":["deposit_count"]}}' -H 'Content-Type: application/json'
```


**Examples of various lottery Smart Contract functions**
**Eg: To play lottery**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":111111,"destination":"deto1qxqqen6lqmksmzmxmfqmxp2y8zvkldtcq8jhkzqflmyczepjw9dp46gc3cczu"}],"scid":"aacaa7bb2388d06e523e5bc0783e4e131738270641406c12978155ba033373af","sc_rpc":[{"name":"entrypoint","datatype":"S","value":"Lottery"}] }}' -H 'Content-Type: application/json'

```

**Eg: Withdraw balance only for smart contract owner**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":111111,"destination":"deto1qxqqen6lqmksmzmxmfqmxp2y8zvkldtcq8jhkzqflmyczepjw9dp46gc3cczu"}],"scid":"aacaa7bb2388d06e523e5bc0783e4e131738270641406c12978155ba033373af", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"Withdraw"}, {"name":"amount","datatype":"U","value":2 }] }}' -H 'Content-Type: application/json'
```

**Eg: To transfer ownership**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":111111,"destination":"deto1qxqqen6lqmksmzmxmfqmxp2y8zvkldtcq8jhkzqflmyczepjw9dp46gc3cczu"}],"scid":"aacaa7bb2388d06e523e5bc0783e4e131738270641406c12978155ba033373af", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"TransferOwnership"}, {"name":"newowner","datatype":"S","value":"deto1qxsplx7vzgydacczw6vnrtfh3fxqcjevyxcvlvl82fs8uykjkmaxgfgulfha5" }] }}' -H 'Content-Type: application/json'

```


```
**Eg: To claim ownership**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":111111,"destination":"deto1qxqqen6lqmksmzmxmfqmxp2y8zvkldtcq8jhkzqflmyczepjw9dp46gc3cczu"}],"scid":"aacaa7bb2388d06e523e5bc0783e4e131738270641406c12978155ba033373af", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"ClaimOwnership"}] }}' -H 'Content-Type: application/json'
```


**Eg: To update code**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{ "transfers":[{"amount":111111,"destination":"deto1qxqqen6lqmksmzmxmfqmxp2y8zvkldtcq8jhkzqflmyczepjw9dp46gc3cczu"}],"scid":"aacaa7bb2388d06e523e5bc0783e4e131738270641406c12978155ba033373af", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"UpdateCode"}, {"name":"code","datatype":"S","value":"new code should be placed here" }] }}' -H 'Content-Type: application/json'

```


