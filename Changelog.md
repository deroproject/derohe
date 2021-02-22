### Welcome to the DEROHE Testnet
[Explorer](https://testnetexplorer.dero.io) [Source](https://github.com/deroproject/derohe) [Twitter](https://twitter.com/DeroProject) [Discord](https://discord.gg/H95TJDp) [Wiki](https://wiki.dero.io) [Github](https://github.com/deroproject/derohe) [DERO CryptoNote Mainnet Stats](http://network.dero.io) [Mainnet WebWallet](https://wallet.dero.io/) 

### DERO HE Changelog  
[From Wikipedia: ](https://en.wikipedia.org/wiki/Homomorphic_encryption) 

###At this point in time, DERO blockchain has the first mover advantage in the following 
  * Private SCs ( no one knows who owns what tokens and who is transferring to whom and how much is being transferred)
  * Homomorphic protocol
  * Ability to do instant sync (takes couple of seconds or minutes), depends on network bandwidth.
  * Ability to deliver encrypted license keys and other data.
  * Pruned chains are the core.
  * Ability to model 99.9% earth based financial model of the world.
  * Privacy by design, backed by crypto algorithms. Many years of research in place.


###3.3 
  * Private SCs are now supported. (90% completed).
  * Sample Token contract is available with guide.
  * Multi-send is now possible. sending to multiple destination per tx
  * Few more ideas implemented and will be tested for review in upcoming technology preview.

###3.2
  * Open SCs are now supported
  * Private SCs which have their balance encrypted at all times (under implementation)
  * SCs can now update themselves. however, new code will only run on next invocation
  * Multi Send is under implementation.

###3.1
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

