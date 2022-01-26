// Asset Interchange/Exchnage  Smart Contract Example in DVM-BASIC.  
//    This SC allows you to deposit an arbitrary token, into it
//    and later allows you to swap one token with another
// if the SC has enough balance to cover outgoing transfer it will be done


    // deposits an arbitrary token
    //  owner should deposits all arbitrary types

   Function Deposit() Uint64 
	20  RETURN 0
	End Function

// incoming represents incoming asset type basicallly an SCID
// outgoing represents outgoing asset type basicallly an SCID
   Function Interchange(incoming String, outgoing String) Uint64
	10  SEND_ASSET_TO_ADDRESS(SIGNER(),ASSETVALUE(incoming)/2, outgoing)   // 1 to 1 interchange of assets
	20  RETURN 0
	End Function
	

	Function Initialize() Uint64
	10 STORE("owner", SIGNER())   // store in DB  ["owner"] = address
	40  RETURN 0 
	End Function 

	
// everything below this is supplementary and not required

	// This function is used to change owner 
	// owner is an string form of address 
	Function TransferOwnership(newowner String) Uint64 
	10  IF LOAD("owner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  STORE("tmpowner",ADDRESS_RAW(newowner))
	40  RETURN 0
	End Function
	
	// Until the new owner claims ownership, existing owner remains owner
        Function ClaimOwnership() Uint64 
	10  IF LOAD("tmpowner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  STORE("owner",SIGNER()) // ownership claim successful
	40  RETURN 0
	End Function
	
	// if signer is owner, withdraw any requested funds
	// if everthing is okay, they will be showing in signers wallet
   Function Withdraw(amount Uint64, asset String) Uint64 
	10  IF LOAD("owner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  SEND_ASSET_TO_ADDRESS(SIGNER(),amount,asset)
	40  RETURN 0
	End Function
	
	// if signer is owner, provide him rights to update code anytime
        // make sure update is always available to SC
        Function UpdateCode( code String) Uint64 
	10  IF LOAD("owner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  UPDATE_SC_CODE(code)
	40  RETURN 0
	End Function
	
	


