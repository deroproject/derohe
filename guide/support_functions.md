Support Functions are inbuilt functions which provide some functionality or expose internals for speed and technical reasons.


LOAD(variable)
==============

LOAD loads a variable which was previously stored in the blockchain using STORE function. Return type will be Uint64/String depending on what is stored.
It will panic  if the value does NOT exists

Uint64 EXISTS(variable)
=======================
EXISTS return 1 if the variable is store in DB and 0 otherwise

STORE(key variable, value variable)
===================================
STORE stores key and value in the DB. All storage state of the SC is accessible only from the  SC which created it.

Uint64 RANDOM()
Uint64 RANDOM(limit Uin64)
============================
RANDOM returns a random using a PRNG seeded on BLID,SCID,TXID. First form gives a uint64, second form returns 
random number in the range 0 - (limit),  0 is inclusive, limit is exclusive

String SCID()
==============
Returns SMART CONTRACT ID which is currently running

String BLID()
==============
Returns current BLOCK ID which contains current execution-in-progress TXID

String TXID()
=============
Returns current TXID which is execution-in-progress.

Uint64 BLOCK_HEIGHT()
=====================
Returns current chain height of BLID()

Uint64  BLOCK_TOPOHEIGHT()
===========================
Returns current topoheight of BLID()

String SIGNER()
=================
Returns address of who signed this transaction

Uint64 IS_ADDRESS_VALID(p String)
=================================
Returns 1 if address is valid, 0 otherwise


String ADDRESS_RAW(p String)
============================
Returns address in RAW form as 33 byte keys, stripping away textual/presentation form. 2 address should always be compared in  RAW form

SEND_DERO_TO_ADDRESS(a String, amount Uint64)
==============================================
Sends amount DERO  from SC DERO balance to a address which should be raw form. address must in string form DERO/DETO form
If the SC does not have enough balance, it will panic


ADD_VALUE(a String, amount Uint64)
====================================
Send specific number of token to specific account.
If account is bring touched for the first time, it is done simply.
If account is already initialized ( it already has some balance, but SC does not know how much). So, it gives additional balance homomorphically


