# Function statement

Function statement is used to define a function. See eg, below for a function which adds  2 numbers

```
Function ADD(x uint64, y uint64) uint64 
10 RETURN x + y
End Function

```

Function syntax is of 2 types

1. Function function_name( 0 or more arguments ) 
2. Function function_name( 0 or more arguments ) return type

The rules for functions are as follows
* All functions begin with Function keyword 
* Function name should be alpha-numeric. If the first letter of the function is Upper case alphabet, it  can be invoked by blockchain and other smart-contracts. Otherwise the function can only be called by other functions within the smart contract.
* Function may or may not have a return type
* All functions must use RETURN to return from function or to return a value. RETURN is mandatory.
* All functions must end with End Function. End Function is mandatory
* A function can have a implicit parameter value of type uint64, which contains amount of DERO value sent with the transaction.

Any error caused during processing will immediately stop  execution and discard all changes that occur during SC execution.

Any Entrypoint which returns uint64  value 0 is termed as success and will make transaction to commit all state changes.
