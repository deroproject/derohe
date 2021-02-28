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


**Download SC Code,check balance and variables from SC**
```curl http://127.0.0.1:40402/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getsc","params":{ "scid":"30b84e9ab5baeee7195e7e1ccb1f533b7402beb2d3cfa97216a6d80c01056f66" , "code":true}}' -H 'Content-Type: application/json'
```
```
curl http://127.0.0.1:40402/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getsc","params":{ "scid":"30b84e9ab5baeee7195e7e1ccb1f533b7402beb2d3cfa97216a6d80c01056f66" , "code":false, "keysstring":["deposit_count"]}}' -H 'Content-Type: application/json'
```


**Examples of various lottery Smart Contract functions**
**Eg: To play lottery**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{"sc_dero_deposit":200000,"scid":"30b84e9ab5baeee7195e7e1ccb1f533b7402beb2d3cfa97216a6d80c01056f66","sc_rpc":[{"name":"entrypoint","datatype":"S","value":"Lottery"}] }}' -H 'Content-Type: application/json'
```
**Note:** Every second deposit/play will trigger the lottery and one wallet will win the lottery.


**Eg: Withdraw balance only for smart contract owner**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{ "scid":"30b84e9ab5baeee7195e7e1ccb1f533b7402beb2d3cfa97216a6d80c01056f66", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"Withdraw"}, {"name":"amount","datatype":"U","value":100000 }] }}' -H 'Content-Type: application/json'
```

**Eg: To transfer ownership**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{ "scid":"30b84e9ab5baeee7195e7e1ccb1f533b7402beb2d3cfa97216a6d80c01056f66", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"TransferOwnership"}, {"name":"newowner","datatype":"S","value":"detoAddressForOwnershipReceiver" }] }}' -H 'Content-Type: application/json'

```

**Eg: To claim ownership**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{ "scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"ClaimOwnership"}] }}' -H 'Content-Type: application/json'
```


**Eg: To update code**
```
curl http://127.0.0.1:40403/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{ "scid":"YourSCID", "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"UpdateCode"}, {"name":"code","datatype":"S","value":"new code should be placed here" }] }}' -H 'Content-Type: application/json'
```
**NOTE:** Please use this update code command carefully. Try this command several times on testnet before issuing on maiinet to update SC code.

