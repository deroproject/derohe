#!/usr/bin/env bash

#set -x   # to enable debug and verbose printing of each and every command

# the SC will trigger panic and but cannot return the funds to sender since ringsize > 2, instead deposits to SC

CURDIR=`/bin/pwd`
BASEDIR=$(dirname $0)
ABSPATH=$(readlink -f $0)
ABSDIR=$(dirname $ABSPATH)

command -v curl >/dev/null 2>&1 || { echo "I require curl but it's not installed.  Aborting." >&2; exit 1; }
command -v jq >/dev/null 2>&1 || { echo "I require jq but it's not installed.  Aborting." >&2; exit 1; }

baseasset="0000000000000000000000000000000000000000000000000000000000000000"

daemon_rpc_port="20000"  # daemon rpc is listening on this port

# we have number of wallets listening at ports from 30000
# we will be using 3 wallets, named owner, player1,player2
owner_rpc_port="30000"
player1_rpc_port="30001"
player2_rpc_port="30002"

owner_address=$(curl --silent http://127.0.0.1:$owner_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getaddress"}' -H 'Content-Type: application/json'| jq -r ".result.address")
player1_address=$(curl --silent http://127.0.0.1:$player1_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getaddress"}' -H 'Content-Type: application/json'| jq -r ".result.address")
player2_address=$(curl --silent http://127.0.0.1:$player2_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getaddress"}' -H 'Content-Type: application/json'| jq -r ".result.address")


function balance(){
  	curl --silent http://127.0.0.1:$1/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getbalance"}' -H 'Content-Type: application/json'| jq -r ".result.balance"
}

function scassetbalance(){
  	curl --silent http://127.0.0.1:$daemon_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getsc","params":{ "scid":"'"$1"'" , "code":false, "variables":true}}' -H 'Content-Type: application/json'| jq -r '.result.balances."'$2'"'
}

function tokenbalance(){
    curl --silent http://127.0.0.1:$1/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getbalance" , "params":{ "scid":"'"$2"'" }}' -H 'Content-Type: application/json'| jq -r ".result.balance"
}

owner_rpc_port="30000"
user1_rpc_port="30001"
user2_rpc_port="30002"

owner_address=$(curl --silent http://127.0.0.1:$owner_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getaddress"}' -H 'Content-Type: application/json'| jq -r ".result.address")
user1_address=$(curl --silent http://127.0.0.1:$user1_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getaddress"}' -H 'Content-Type: application/json'| jq -r ".result.address")
user2_address=$(curl --silent http://127.0.0.1:$user2_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"getaddress"}' -H 'Content-Type: application/json'| jq -r ".result.address")


# use owner wallet to load/install an lotter sc to chain
exchangescid=$(curl --silent --request POST --data-binary   @asset_exchange.bas http://127.0.0.1:$owner_rpc_port/install_sc| jq -r ".txid")
echo "exchange SCID" $exchangescid
sleep 2

asset1scid=$(curl --silent --request POST --data-binary   @asset.bas http://127.0.0.1:$owner_rpc_port/install_sc| jq -r ".txid")
echo "asset1 SCID" $asset1scid
sleep 2


asset2scid=$(curl --silent --request POST --data-binary   @asset.bas http://127.0.0.1:$owner_rpc_port/install_sc| jq -r ".txid")
echo "asset2 SCID" $asset2scid
sleep 2


echo -n "owner exchanging dero 1000 for asset1 "
curl --silent http://127.0.0.1:$owner_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{"sc_dero_deposit":1000,"scid":"'"$asset1scid"'","ringsize":2, "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"IssueAsset"}] }}' -H 'Content-Type: application/json' | jq -r ".result.txid"
sleep 2
echo -n "owner exchanging dero 1000 for asset2 "
curl --silent http://127.0.0.1:$owner_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{"sc_dero_deposit":1000,"scid":"'"$asset2scid"'","ringsize":2, "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"IssueAsset"}] }}' -H 'Content-Type: application/json' | jq -r ".result.txid"
sleep 2


echo -n "owner depositing triple assets ( dero, asset1, asset2 ) to exchangescid "
curl --silent http://127.0.0.1:$owner_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{"scid":"'"$exchangescid"'","ringsize":2, "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"Deposit"}], "transfers": [{"burn":1234,"destination":"'"$user2_address"'"}, {"scid":"'"$asset1scid"'", "burn":123}, {"scid":"'"$asset2scid"'", "burn":555}] }}' -H 'Content-Type: application/json' | jq -r ".result.txid"
sleep 2


echo "SC DERO balance" $(scassetbalance $exchangescid $baseasset )
echo "SC Asset1 balance" $(scassetbalance $exchangescid $asset1scid )
echo "SC Asset2 balance" $(scassetbalance $exchangescid $asset2scid )


if [[ $(scassetbalance $exchangescid $baseasset) -ne 1234  ||  $(scassetbalance $exchangescid $asset1scid) -ne 123 ||  $(scassetbalance $exchangescid $asset2scid) -ne 555 ]] ; then
    echo "condition failed"
    exit 1 
fi


echo -n "user1 exchanging dero 2000 for asset1 "
curl --silent http://127.0.0.1:$user1_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{"sc_dero_deposit":2000,"scid":"'"$asset1scid"'","ringsize":2, "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"IssueAsset"}] }}' -H 'Content-Type: application/json' | jq -r ".result.txid"
sleep 2
echo  "wallet1 received asset1 tokens in return" $(tokenbalance $user1_rpc_port $asset1scid)


echo -n "user1 depositing 122 asset1 to obtain 61 asset2"
curl --silent http://127.0.0.1:$user1_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"transfer","params":{"scid":"'"$exchangescid"'","ringsize":2, "sc_rpc":[{"name":"entrypoint","datatype":"S","value":"Interchange"},{"name":"incoming","datatype":"H","value":"'"$asset1scid"'"},{"name":"outgoing","datatype":"H","value":"'"$asset2scid"'"}], "transfers": [{"scid":"'"$asset1scid"'", "burn":122}] }}' -H 'Content-Type: application/json' | jq -r ".result.txid"
#curl --silent http://127.0.0.1:$user1_rpc_port/json_rpc -d '{"jsonrpc":"2.0","id":"0","method":"scinvoke","params":{"sc_dero_deposit":10000,"scid":"'"$exchangescid"'","ringsize":2, "sc_rpc":[] }}' -H 'Content-Type: application/json' | jq -r ".result.txid"


sleep 2
echo  "wallet1  asset1 tokens " $(tokenbalance $user1_rpc_port $asset1scid)
echo  "wallet1  asset2 tokens after exchange " $(tokenbalance $user1_rpc_port $asset2scid)

if [[ $(tokenbalance $user1_rpc_port $asset1scid) -ne 1878  ||  $(tokenbalance $user1_rpc_port $asset2scid) -ne 61 ]] ; then
    echo "condition failed"
    exit 1 
fi

exit 0