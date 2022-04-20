/* Name Service SMART CONTRACT in DVM-BASIC.  
   Allows a user to register names which could be looked by wallets for easy to use name while transfer
*/


 // This function is used to initialize parameters during install time
	Function Initialize() Uint64
	10  RETURN 0 
	End Function 

    // Register a name, limit names of 5 or less length
    Function Register(name String) Uint64 
	10  IF EXISTS(name) THEN GOTO 50   // if name is already used, it cannot reregistered
	15  IF STRLEN(name) >= 64 THEN GOTO 50 // skip names misuse
	20  IF STRLEN(name) >= 6 THEN GOTO 40 
	30  IF SIGNER() == address_raw("deto1qyvyeyzrcm2fzf6kyq7egkes2ufgny5xn77y6typhfx9s7w3mvyd5qqynr5hx") THEN GOTO 40
	35  IF SIGNER() != address_raw("deto1qy0ehnqjpr0wxqnknyc66du2fsxyktppkr8m8e6jvplp954klfjz2qqdzcd8p") THEN GOTO 50 
	40  STORE(name,SIGNER())
	50  RETURN 0
	End Function

  	
	// This function is used to change owner of Name is an string form of address 
	Function TransferOwnership(name String,newowner String) Uint64 
	10  IF LOAD(name) != SIGNER() THEN GOTO 30 
	20  STORE(name,ADDRESS_RAW(newowner))
	30  RETURN 0
	End Function


	// This function is used to change SC owner 
	Function TransferSCOwnership(newowner String) Uint64 
	10  IF LOAD("owner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  STORE("own1",ADDRESS_RAW(newowner))
	40  RETURN 0
	End Function
	
	// Until the new owner claims ownership, existing owner remains owner
    Function ClaimSCOwnership() Uint64 
	10  IF LOAD("own1") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  STORE("owner",SIGNER()) // ownership claim successful
	40  RETURN 0
	End Function
	
	// If signer is owner, provide him rights to update code anytime
    // make sure update is always available to SC
    Function UpdateCode(SC_CODE String) Uint64 
	10  IF LOAD("owner") == SIGNER() THEN GOTO 30 
	20  RETURN 1
	30  UPDATE_SC_CODE(SC_CODE)
	40  RETURN 0
	End Function
