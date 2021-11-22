package rpc

// definitions related to SC

type SC_ACTION uint64 // sc actions are coded as follow
const (
	SC_CALL SC_ACTION = iota
	SC_INSTALL
)

// SC_CALL must have an arg of type SC_ENTRYPOINT of type String, SCID of type Hash
// SC_INSTALL must have an arg SC of String

// some fields are already defined
const SCACTION = "SC_ACTION" //all SCS must have an ACTION
const SCCODE = "SC_CODE"     // SCCODE must be sent in this ARGUMENT
const SCSIGNER = "SC_SIGNER" // the signer address
const SCSIGNC = "SC_SIGNC"   // the sign C component
const SCSIGNS = "SC_SIGNS"   // the sign S component

const SCID = "SC_ID" // SCID
