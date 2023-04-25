### DERO HE Changelog

### Release 117
 * Out of memory bug fix(Reported by Slixe)

### Release 116
 * Added miniblock spam fix(Bug reported by Slixe)
 * Pruned chain can be rewinded till its pruned height only(Bug reported by Slixe)
 * Added a wallet crash fix
 * Fixed github issue 112
 * Fixed github issue 98


### At this point in time, DERO blockchain has the first mover advantage in the following 

* Private SCs ( no one knows who owns what tokens and who is transferring to whom and how much is being transferred.)
* Homomorphic protocol
* Ability to do instant sync (takes couple of seconds or minutes), depends on network bandwidth.
* DAG/MINIDAG with 1 miniblock every second
* Mining Decentralization.No more mining pools, daily 100000 reward blocks, no need for pools and thus no attacks
* Erasure coded blocks, lower bandwidth requirements, very low propagation time.
* Ability to deliver encrypted license keys and other data.
* Pruned chains are the core.
* Ability to model 99.9% earth based financial model of the world.
* Privacy by design, backed by crypto algorithms. Many years of research in place.
- Sample Token contract is available with guide.
- Multi-send is now possible. sending to multiple destination per tx
- DERO Simulator for faster development/testing
- Few more ideas implemented and will be tested for review in upcoming technology preview.



### 3.4

- DAG/MINIDAG with blocks flowing every second
- Mining Decentralization.No more mining pools, daily 100000 reward blocks, no need for pools and thus no attacks
- Erasure coded blocks, lower bandwidth requirements, very low propagation time. Tested with upto 20 MB blocks.
- DERO Simulator for faster Development cycle 
- Gas Support implemented ( both storage gas/compute gas)
- Implemented gas estimation
- DVM simulator to test all edge cases for SC dev, see dvm/simulator_test.go to see it in action for lotter SC. 

### 3.3 

* Private SCs are now supported. (90% completed).
* Sample Token contract is available with guide.
* Multi-send is now possible. sending to multiple destination per tx
* Few more ideas implemented and will be tested for review in upcoming technology preview.

### 3.2

* Open SCs are now supported
* Private SCs which have their balance encrypted at all times (under implementation)
* SCs can now update themselves. however, new code will only run on next invocation
* Multi Send is under implementation.

### 3.1

* TX now have significant savings of around 31 * ringsize bytes for every tx
* Daemon now supports pruned chains.
* Daemon by default bootstraps a pruned chain.
* Daemon currently syncs full node by using --fullnode option.
* P2P has been rewritten for various improvements and easier understanding of state machine
* Address specification now enables to embed various RPC parameters for easier transaction
* DERO blockchain represents transaction finality  in a couple of blocks (less than 1 minute), unlike other blockchains.
* Proving and parsing of embedded data is now available in explorer.
* Senders/Receivers both have proofs which confirm data sent on execution.
* All tx now have inbuilt space of 144 bytes for user defined data
* User defined space has inbuilt RPC which can be used to implement most practical use-cases.All user defined data is encrypted.
* The model currrently defines data on chain while execution is referred to wallet extensions. A dummy example of pongserver extension showcases how to enable purchases/delivery of license keys/information privately.
* Burn transactions which burn value are now working.

###3.0

* DERO HE implemented


