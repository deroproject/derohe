## Sign a transaction using an offline wallet
This README matches the behaviour of a modified release 140

## Introduction
The problem with traditional wallets and how Dero addresses these
shortfalls:
1. How traditional wallets work<br>
   A wallet file (wallet.db) stores your addresses. Each address consists of a public
   and secret part. The public part is what you share with people, so they can
   make payments to you. The secret key is required to unlock and spend the funds
   associated with that address.
   
2. Risk associated with traditional wallets<br>
   If somebody obtains your secret key they can spend your funds. The network will
   process the transaction if the cryptographic signature is correct. The network
   doesn't validate who sent the transaction or from where.
   An attacker will aim to obtain your wallet file in order to steal your secret
   keys, and per extension, your funds. Malware hidden in applications is an easy
   way for hackers to scout your hard drive for wallet files, which they send to
   themselves over your internet link.<br>
   
   Apart from the risk of loosing your funds to an attacker it's also much more likely
   you'll loose your wallet file due to neglect. There are countless horror stories
   of people who have lost billions (yes, with a 'B') worth of Bitcoin due to
   hard drive crashes, hard drives that were formatted to make room for a new
   O/S or game installation or simply old machines that were thrown away.<br>
   
   If you were diligent and made a backup of the wallet file, you have to remember
   which addresses it contains. Each time you create a new addresses it is not 
   automatically added to the backup copy. 
   Upon restoring of an old backup you might discover that it doesn't contain all 
   the new addresses created since the backup. Access to those funds are lost forever.<br>
   
3. BIP39: Mnemonic phrase improvement<br>
   The problem of loosing addresses and their private keys were mitigated with the
   implementation of BIP39 mnemonic seed phrases. A single master key is created.
   All the addresses in that wallet are derived from the master key. To the user
   the master key is presented as a random 25 word (natural language) phrase, 
   called a mnemonic. 
   
   When the wallet is created this phrase must be written down and stored in a secure
   manner. The storage medium must protect against fire and moisture damage.
   
   Do not use your phone to take a picture of the phrase. Synchronisation software 
   could upload your images to the 'cloud' where it can be intercepted in transit
   or on the remote storage. Do not print it out either. In rare cases malware in
   printer firmware has been found to recognise these seed phrases and send it of
   to the hackers.
   
   Paranoia regarding protection of your seed phrase is not unwarrented.
   
   When it's time to restore your addresses in a new wallet all you have to do is
   to type in the mnemonic. The application will convert it to the master key.
   No software backup of the wallet file is required. By initialising the wallet
   with a valid seed phrase all the addresses derived from it can be recreated.
   
   If you have funds in a traditional wallet thats not based on BIP39, you can
   setup a second wallet that uses the BIP39 technology. You can pay the funds
   from the old wallet addresses to the new wallet, thereby transferring the 
   funds to the new wallet.
   
4. Encrypted wallets
   As seen above, BIP39 guards against loosing access to your individual addresses.
   It however does not provide additional protection of loosing the actual wallet
   file to an attacker. 
   On Dero the wallet file is encrypted with a password. If an attacker obtains
   the wallet file, they will not be able to open it. Your security is based on
   choosing a long, secure password which can withstand a brute force attack. 
      
5. Offline cold storage<br>
   Two computers are required for this setup. One is connected to the internet
   while the other has no network connectivity. 

   The machine without network access is called the offline machine. On it you
   setup an encrypted wallet with mnemonic seed phrase. The machine doesn't have
   a copy of the blockchain. This wallet will perform transaction signing (authorisation).

   The machine with internet access is called the online machine. This machine
   is synchronised with the blockchain. This wallet is called the viewing wallet. It displays
   your balance and transaction history and can compile a transaction, but not authorise it.<br>
   
   This process adds two levels of complexity:<br>
   *  How do you view the transactions & balance of an address located in the offline wallet?<br>
   *  How do you spend the funds of an address located in the offline wallet?<br>
   <br>
   The Dero command line (CLI) wallet overcomes these obstacles as follow:<br>
   
   <b>Viewing the transaction history</b><br>
   The Dero blockchain is encrypted. None of the transaction data is visible on
   the blockchain. You can't import an address into the online wallet and view
   the activity that occurred on that address. The secret key is required to 
   decrypt the data that contains your transactions.

   Dero requires that your register your address on the network. This allows an
   initial filter of the data so that your wallet client only receives data that
   is related to its address.

   As the online wallet receives the data it will create a separate data file, which
   needs to be copied to the offline wallet for decryption. The decrypted data must
   be copied back to the online wallet where it is imported. The decrypted data
   enables the wallet to extract the transaction data and to compile your account balance.
   
   If the online wallet is somehow stolen it's (almost) no deal - information
   is leaked regarding the balance & transaction history of your address. This is
   clearly not a desireable thing, but at least the attacker will not be able to spend
   the funds, like they would have if the wallet contained the secret (private)
   key as well.
   
   <b>Spend the funds</b><br>
   The online wallet can construct a transaction. All the required inputs are contained in this transaction.
   The transaction is stored in a data file. The file must be copied to the offline machine. There the secret
   key authorises (signs) the transaction. The authorised transaction is copied back to the online wallet and
   submitted to the network for processing.
   
6. Summary
   The Dero CLI wallet gives you access to these features today:
   * BIP39 master seed 
   * Split addresses between two wallets. The offline contains the private keys
     and the online the viewing and spending keys.
   * Encrypted storage of the wallet on disk and encrypted communication
   * All running on an encrypted blockchain

## Software setup
1. You require two PCs, preferrably Intel i5 or faster. About 200mb of hard drive space
   is required if you plan to connect to a remote node. The internet link speed isn't 
   very important if you connect to a remote node. 1mbps or faster is sufficient.
   The machines can run Linux, Mac OS X or MS Windows. For this tutorial Linux is used.
   
2. You can obtain the software from the official Dero website:
   https://dero.io/download.html or a source copy from GitHUB at
   https://github.com/deroproject/derohe
   
   At the moment only the cli (command line) wallet supports offline transaction signing.

   If you've downloaded the Linux CLI (command line interface) install archive, extract
   it as follow:<br><i>
   $ tar -xvzf dero_linux_amd64.tar.gz<br>
   $ cd dero_linux_amd64<br>
   $ ls<br>
   derod-linux-amd64  dero-miner-linux-amd64  dero-wallet-cli-linux-amd64  explorer-linux-amd64  simulator-linux-amd64  Start.md<br>
   </i><br>
   We'll use the dero-wallet-cli-* application
   
   To build from source, you'll need to Go language (golang) compiler on your machine
   to compile the software. On a Debian based Linux installation, you can install the
   package as follow:<br><i>
   $ sudo apt-get update<br>
   $ sudo apt-get install golang:amd64</i><br>
      
   If you want to check out a copy of the github source code:<br><i>
   $ git clone https://github.com/deroproject/derohe<br>
   $ cd derohe/cmd/dero-wallet-cli<br>
   $ go build</i><br> 
   The new application is called: dero-wallet-cli<br>
3. Offline machine with the signing wallet<br>
3.1. First run<br>
   From a terminal console, launch the application: <i>./dero-wallet-cli --help</i><br>
   We will use the following command line options:<br>
   <i>--offline</i> - Specify that this wallet is an offline (signing) wallet<br>
   <i>--wallet-file</i> - The name of your wallet, i.e. offline.db<br>
   <i>--password</i> - The password with which to encypt the wallet. It needs to be a strong password, 
   which can withstand a password attack, but note, you'll have to enter this password regularly, so it
   still needs to be something practical to work with.<br>
   <i>--generate-new-wallet</i> - Let the wallet create a new mnemonic seed phrase and address<br>
   or<br>
   <i>--restore-deterministic-wallet</i> - You'll provide the mnemonic seed phrase<br>
   <i>--electrum-seed</i> - Here you'll provide the mnemonic phrase<br>
   An example will be:<br>
   <i>$ ./dero-wallet-cli --offline --wallet-file=offline.db --password=someexamplepw --restore-deterministic-wallet --electrum-seed="your 25 seedphrase words here"</i><br>
   After the wallet starts up the menu will provide you with a couple of options. At the top of the menu is a greeting to show you that it is running in offline mode:<br>
&nbsp;Offline (signing) wallet:<br>
&nbsp;&nbsp;&nbsp;1. Exported public key to setup the online (view only) wallet<br>
&nbsp;&nbsp;&nbsp;2. Generate a registration transaction for the online wallet<br>
&nbsp;&nbsp;&nbsp;3. Sign spend transactions for the online wallet<br>
   Select '0' to exit the wallet.<br>
3.2. Second run<br>
   Now that the wallet is already created, you don't provide the restore & seed CLI options anymore:<br>
   <i>$ ./dero-wallet-cli --offline --wallet-file=offline.db --password=someexamplepw</i><br>
   Menu options:<br>
   1 Display account Address<br>
     Display your account address. Share this with people so they can send Dero to you:<br>
     &nbsp;&nbsp;<i>Wallet address : dero1abcdef12345678907j0n6ft4yzlm300fxzz2sg84t28g2cp897f5yqghyx4z3</i><br>
   2 Display seed<br>
     This prints your mnemonic recovery seed. If somebody obtains this seed phrase, they can restore a wallet and spend all your funds.<br>
   3 Display Keys<br>
     This normally contains the public and secret keys. While you're running in offline mode, an additional entry is display, the 'view only' key. This key is used to set up the online (view only) wallet.<br>
     &nbsp;&nbsp;<i>secret key: &lt;Your secret key&gt;</i><br>
     &nbsp;&nbsp;<i>public key: &lt;Your public key&gt;</i><br>
     &nbsp;&nbsp;<i>View only key - Import the complete text into the online (view only) wallet to set it up:</i><br>
     &nbsp;&nbsp;<i>viewkey,&lt;key parts&gt;;20010</i><br>
   4 Generate registration transaction<br>
     In order to use a remote node, i.e. running in 'light mode', where you do not download the full blockchain yourself, the node requires you to register your address on it.<br>
     Example output:<br>
     &nbsp;&nbsp;&nbsp;<i>Generating registration transaction for wallet address : dero1&lt;Your address&gt;<br>
     &nbsp;&nbsp;&nbsp;Searched 100000 hashes<br>
     &nbsp;&nbsp;&nbsp;...<br>
     &nbsp;&nbsp;&nbsp;...<br>
     &nbsp;&nbsp;&nbsp;Searched 24600000 hashes<br>
     &nbsp;&nbsp;&nbsp;Found transaction:<br>
     &nbsp;&nbsp;&nbsp;Found the registration transaction. Import the complete text into the online (view only) wallet:<br>
     &nbsp;&nbsp;&nbsp;registration,dero1&lt;registration text&gt;;24578</i><br>
4. Online machine with the view only wallet<br>
4.1 First run<br>
  From a terminal console, launch the application: <i>./dero-wallet-cli --help</i><br>
  We will use the following command line options:<br>
  <i>--remote</i> - Connect to a remote node. This is often called 'light weight mode', since you do not maintain a full copy of the blockchain.<br>
  <i>--wallet-file</i> - The name of your wallet, i.e. viewonly.db<br>
  <i>--password</i> - The password with which to encypt the wallet. It needs to be a strong password, which can withstand a password attack, but note, you'll have to enter this password regularly, so it still needs to be something practical to work with.<br>
  <i>--restore-viewonly-wallet</i> - Set up the wallet with the viewing key obtained from the offline wallet<br>
An example will be:<br>
  <i>$ ./dero-wallet-cli --remote --wallet-file=viewonly.db --password=someexamplepw --restore-viewonly-wallet</i><br>
  The software will have these 2 prompts:<br>
  &nbsp;&nbsp;&nbsp;<i>Enter wallet filename (default viewonly.db):</i> Just press enter to accept the default<br>
  &nbsp;&nbsp;&nbsp;<i>Enter the view only key (obtained from the offline (signing) wallet):</i> Paste the viewing key here<br>
If the key was accepted, you'll get this confirmation:<br>
&nbsp;&nbsp;&nbsp;Successfully restored an online (view only) wallet<br>
&nbsp;&nbsp;&nbsp;Address: dero1&lt;Your address&gt;<br>
&nbsp;&nbsp;&nbsp;Public key: &lt;Your public key&gt;<br>
After the wallet starts up the menu will provide you with a couple of options. At the top of the menu is a greeting to show you that it is running in view only mode:<br>
&nbsp;Online (view only) wallet:<br>
&nbsp;&nbsp;&nbsp;1. Register you account, using registration transaction from the offline (signing) wallet.<br>
&nbsp;&nbsp;&nbsp;2. View your account balance & transaction history<br>
&nbsp;&nbsp;&nbsp;3. Generate transactions for the offline wallet to sign.<br>
&nbsp;&nbsp;&nbsp;4. Submit the signed transactions to the network.<br>
  Select '0' to exit the wallet.<br>
4.2 Second run<br>
  Now that the wallet is already created, you don't provide the restore & seed CLI options anymore:<br>
  <i>$ ./dero-wallet-cli --remote --wallet-file=viewonly.db --password=someexamplepw</i><br>
   Menu options:<br>
   1 Display account Address<br>
     Display your account address. Share this with people so they can pay you. This address must match the address in the offline (signing) wallet.<br>
     &nbsp;&nbsp;<i>Wallet address : dero1abcdef12345678907j0n6ft4yzlm300fxzz2sg84t28g2cp897f5yqghyx4z3</i><br>
   2 Display seed<br>
     This option is not available in the view only wallet.<br>
   3 Display Keys<br>
     Only the public key is displayed. This must match the public key in the offline wallet<br>
   4 Account registration to blockchain<br>
     In order to use a remote node, i.e. running in 'light mode', where you do not download the full blockchain yourself, the node requires you to register your address.<br>
     &nbsp;&nbsp;<i>Enter the registration transaction (obtained from the offline (signing) wallet): </i> Paste the registration transaction here<br>
     &nbsp;&nbsp;<i>Registration TXID &lt;txid number&gt;<br>
     &nbsp;&nbsp;registration tx dispatched successfully</i><br>
     Note: After the account was registered, the wallet needs to synchronise your account balance. In order to accomplish this, interaction between the online & offline wallet is required.<br>
## Using the split Online / Offline wallet configuration
  For the demonstration to work effectively, fund your newly created address with some Dero by sending some cents (0.xx) to it, either from an online exchange or from one of your existing wallets with a balance.<br>
1. Balance enquiry and transaction history<br>
  The online (view only) wallet connects to the remote node and retrieves the blockchain data that matches your wallet address. The data needs to be decrypted before the information can be processed. The secret key, located in the offline (signing) wallet, is required to accomplish this.<br>
  Each time the online wallet receives a block which contains transaction information for your address, a part of the information needs to be send to the offline wallet for decryption.<br>
  The online wallet will automatically create a file called 'offline_request' in the <i>prefix</i> directory. The prefix is specified as a command line option when the application is started, i.e.: $ <i>./dero-wallet-cli --prefix=/tmp</i>, along with the other command line options passed to the application.<br><br>
  For testing purposes, if you run the online & offline wallets on the same machine with the same prefix, the file created by the online wallet (offline_request) will be immediately detected by the offline (signing) wallet and automatically processed. If you run a production setup where the two wallets are on separate computers, you'll have to copy the file (offline_request) manually from the online machine to the offline machine.<br>
  The prompts are as follow:<br>
  Online wallet:<br>
  The blockchain interaction occurs in the background. After a transaction applicable to your address is detected this text will appear:<br>
  &nbsp;&nbsp;<i>Interaction with offline wallet required. Saved request to: /prefix/offline_request<br>
  &nbsp;&nbsp;Waiting 60 seconds for the response at: /prefix/offline_response</i><br>
  The 'offline_request' file must be copied to the offline wallet.<br>
  Offline wallet:<br>
  After the 'offline_request' file is copied to the specified prefix directory, the wallet will detect the presense of the file automatically and process it. The wallet reports:<br>
  &nbsp;&nbsp;<i>Found /prefix/offline_request -- new decryption request<br>
  &nbsp;&nbsp;Saved result in /prefix/offline_response</i><br>
  The 'offline_response' file must be copied back to the online wallet.<br>
  Online wallet:<br>
  As soon as the response file is detected the wallet reports:<br>
  &nbsp;&nbsp;<i>Found a valid response</i><br>
  This exchange happens for each transaction that you receive or spend on your address. Your balance will be shown as part of the command prompt. You can view the transaction history under menu entry <i>13 Show transaction history</i>.<br>
  If you use a Pirate+ hardware wallet the data exchange happens in the background. No manual copying of files are required.<br>
2. Spend transaction<br>
  In order to spend your hard earned Dero you first need to fund your address, as suggested at the top of this chapter. The online (view only) wallet will pick up the transaction from the remote node. The transaction history and account balance will be updated, after you've decrypted the data files, as described above in 5.1.<br>
  To create a transaction the prompts are as follow:<br>
  Online wallet: <br>
    Select option <i>5 Prepare (DERO) transaction (for the offline wallet to sign)</i> to prepare the transaction.<br>
    Enter the destination address and amount. The destination port and comment are optional<br>
  After the transaction is confirmed, the wallet prepares all the data and saves it to <i>/prefix/offline_request</i>.<br>
  This file must be copied to the offline wallet and placed in its <i>/prefix</i> directory.<br>
  Offline wallet:<br>
    As soon as the wallet detects the offline request file and determine that it is a transaction that must be signed, a
    prompt is presented:<br>
    &nbsp;&nbsp;<i>Found /tmp/offline_request -- new decryption request<br>
    &nbsp;&nbsp;Detected new transaction sign request. Authorise the request with menu option '5: Sign'.</i><br>
    Enter '5' on the prompt to tell the wallet to sign the transaction. The expected feedback is:<br>
    &nbsp;&nbsp;<i>Read XXXX bytes from /prefix/transaction<br>
    &nbsp;&nbsp;Saved result in /prefix/offline_response</i><br>
    Return the file to the online (view only) wallet to complete the transaction.<br>
  Online wallet:<br>
  As per instruction, return <i>offline_response</i> to the online wallet. When the wallet detect the file it will print this message:<br>
  &nbsp;&nbsp;<i>Read XXXX bytes from /prefix/offline_response<br>
  &nbsp;&nbsp;Ready to broadcast the transaction<br>
  &nbsp;&nbsp;INFO wallet Dispatched tx {"txid": "&lt;tx id&gt;"}</i><br>
Congratulations, you've successfully created and send a transaction using an online/offline split wallet
