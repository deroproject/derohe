package rpc

import (
	"encoding/json"
	"fmt"
	"sort"
	"time"

	"github.com/deroproject/derohe/cryptography/crypto"
	"github.com/fxamacker/cbor/v2"
)

// this package defines interfaces and necessary glue code Digital Network, it exposes and provides encrypted RPC calls over DERO chain

var enc_options = cbor.EncOptions{
	Sort:          cbor.SortCoreDeterministic,
	ShortestFloat: cbor.ShortestFloat16,
	NaNConvert:    cbor.NaNConvert7e00,
	InfConvert:    cbor.InfConvertFloat16,
	IndefLength:   cbor.IndefLengthForbidden,
	TimeTag:       cbor.EncTagRequired,
}

var dec_options = cbor.DecOptions{
	TimeTag: cbor.DecTagRequired,
}

var dec cbor.DecMode
var enc cbor.EncMode

func init() {
	var err error
	if dec, err = dec_options.DecMode(); err != nil {
		panic(err)
	}
	if enc, err = enc_options.EncMode(); err != nil {
		panic(err)
	}

}

// currently we support only the following Data Type
// the following data types are present
// int64 represented by inputbox
// uint64 represented by inputbox
// string represented by input box
// what about listbox, checkbox , checkbox can be represented by bool but currently not suported
type DataType string

const (
	DataString  DataType = "S"
	DataInt64            = "I"
	DataUint64           = "U"
	DataFloat64          = "F"
	DataHash             = "H" // a 256 bit hash (basically sha256 of 32 bytes long)
	DataAddress          = "A" // dero address represented in 33 bytes
	DataTime             = "T"
)

func (d DataType) String() string {
	switch d {
	case DataString:
		return "string"
	case DataInt64:
		return "int64"
	case DataUint64:
		return "uint64"
	case DataFloat64:
		return "float64"
	case DataHash:
		return "hash"
	case DataAddress:
		return "address"
	case DataTime:
		return "time"

	default:
		return "unknown data type"
	}

}

// type  DataType byte
type Argument struct {
	Name     string      `json:"name"`     // string name must be atleast 1 byte
	DataType DataType    `json:"datatype"` // Type must one of the known data types
	Value    interface{} `json:"value"`    // value should be as per type
}

type Arguments []Argument

func (arg Argument) String() string {
	switch arg.DataType {
	case DataString:
		return fmt.Sprintf("Name:%s Type:%s Value:'%s'", arg.Name, arg.DataType, arg.Value)
	case DataInt64:
		return fmt.Sprintf("Name:%s Type:%s Value:'%d'", arg.Name, arg.DataType, arg.Value)
	case DataUint64:
		return fmt.Sprintf("Name:%s Type:%s Value:'%d'", arg.Name, arg.DataType, arg.Value)
	case DataFloat64:
		return fmt.Sprintf("Name:%s Type:%s Value:'%f'", arg.Name, arg.DataType, arg.Value)
	case DataHash:
		return fmt.Sprintf("Name:%s Type:%s Value:'%s'", arg.Name, arg.DataType, arg.Value)
	case DataAddress:
		return fmt.Sprintf("Name:%s Type:%s Value:'%s'", arg.Name, arg.DataType, arg.Value)
	case DataTime:
		return fmt.Sprintf("Name:%s Type:%s Value:'%s'", arg.Name, arg.DataType, arg.Value)

	default:
		return "unknown data type"
	}
}

// tells whether the arguments have an argument of this type
func (args Arguments) Has(name string, dtype DataType) bool {
	for _, arg := range args {
		if arg.Name == name && arg.DataType == dtype {
			return true
		}
	}
	return false
}

// tells whether the arguments have an argument of this type and value it not nil
func (args Arguments) HasValue(name string, dtype DataType) bool {
	for _, arg := range args {
		if arg.Name == name && arg.DataType == dtype && arg.Value != nil {
			return true
		}
	}
	return false
}

// tells the index of the specific argument
func (args Arguments) Index(name string, dtype DataType) int {
	for i, arg := range args {
		if arg.Name == name && arg.DataType == dtype {
			return i
		}
	}
	return -1
}

// return value wrapped in an interface
func (args Arguments) Value(name string, dtype DataType) interface{} {
	for _, arg := range args {
		if arg.Name == name && arg.DataType == dtype {
			return arg.Value
		}
	}
	return nil
}

// this function will pack the args into buffer of specific limit, if it fails, it panics
func (args Arguments) MustPack(limit int) []byte {
	if packed, err := args.CheckPack(limit); err != nil {
		panic(err)
	} else {
		return packed
	}
}

// this function will pack the args into buffer of specific limit, if it fails, it gives eroor
func (args Arguments) CheckPack(limit int) ([]byte, error) {
	packed, err := args.MarshalBinary()
	if err != nil {
		return nil, err
	}
	if len(packed) > limit {
		return nil, fmt.Errorf("Packed size %d bytes, but limit is %d", len(packed), limit)
	}
	if len(packed) == limit {
		return packed, nil
	} else { // we need to fill with 0 values, upto limit size

		fill_count := limit - len(packed)
		for i := 0; i < fill_count; i++ {
			packed = append(packed, 0)
		}

	}

	return packed, nil
}

// pack more deeply
func (args Arguments) MarshalBinary() (data []byte, err error) {
	if err = args.Validate_Arguments(); err != nil {
		return
	}

	localmap := map[string]interface{}{} // this also filters any duplicates
	for _, arg := range args {
		switch v := arg.Value.(type) {
		case int64:
			localmap[arg.Name+string(arg.DataType)] = v
		case uint64:
			localmap[arg.Name+string(arg.DataType)] = v
		case float64:
			localmap[arg.Name+string(arg.DataType)] = v
		case crypto.Hash:
			localmap[arg.Name+string(arg.DataType)] = v
		case Address:
			localmap[arg.Name+string(arg.DataType)] = v.PublicKey.EncodeCompressed()
		case string:
			localmap[arg.Name+string(arg.DataType)] = v
		case time.Time:
			localmap[arg.Name+string(arg.DataType)] = v
		default:
			err = fmt.Errorf("I don't know about type %T!\n", v)
			return
		}
	}
	return enc.Marshal(localmap)
}

func (args *Arguments) UnmarshalBinary(data []byte) (err error) {
	localmap := map[string]interface{}{}

	if err = dec.Unmarshal(data, &localmap); err != nil {
		return err
	}

	*args = (*args)[:0]

	for k, v := range localmap {
		if len(k) < 2 {
			return fmt.Errorf("Invalid encoding for key '%s'", k)
		}

		arg := Argument{Name: string(k[:len(k)-1]), DataType: DataType(k[len(k)-1:])}

		switch arg.DataType {
		case DataInt64:
			if value, ok := v.(int64); ok {
				arg.Value = value
			} else if value, ok := v.(uint64); ok {
				arg.Value = int64(value)
			} else {
				return fmt.Errorf("%+v has invalid data typei %T\n", arg, v)
			}
		case DataUint64:
			if value, ok := v.(uint64); ok {
				arg.Value = value
			} else {
				return fmt.Errorf("%+v has invalid data type %T\n", arg, v)
			}
		case DataFloat64:
			if value, ok := v.(float64); ok {
				arg.Value = value
			} else {
				return fmt.Errorf("%+v has invalid data type %T\n", arg, v)
			}
		case DataHash:
			if value, ok := v.([]uint8); ok {
				var hash crypto.Hash
				copy(hash[:], value)
				arg.Value = hash
			} else {
				return fmt.Errorf("%+v has invalid data type %T\n", arg, v)
			}
		case DataAddress:
			if value, ok := v.([]uint8); ok {
				a := make([]byte, 33, 33)
				copy(a[:], value)

				p := new(crypto.Point)
				if err = p.DecodeCompressed(a[0:33]); err != nil {
					return err
				}
				addr := NewAddressFromKeys(p)
				arg.Value = *addr
			} else {
				return fmt.Errorf("%+v has invalid data type %T\n", arg, v)
			}
		case DataString:
			if value, ok := v.(string); ok {
				arg.Value = value
			} else {
				return fmt.Errorf("%+v has invalid data type %T\n", arg, v)
			}
		case DataTime:
			if value, ok := v.(time.Time); ok {
				arg.Value = value
			} else {
				return fmt.Errorf("%+v has invalid data type %T\n", arg, v)
			}
		default:
			err = fmt.Errorf("I don't know about typeaa %T  %s!\n", v, k)
			return
		}
		*args = append(*args, arg)

	}

	if err = args.Validate_Arguments(); err != nil {
		return
	}
	args.Sort() // sort everything

	return
}

// used to validata  arguments whether the type is proper
func (args Arguments) Validate_Arguments() error {
	for _, arg := range args {
		if len(arg.Name) < 1 {
			return fmt.Errorf("Name must be atleast 1 char long")
		}

		switch arg.DataType {
		case DataString:
			if _, ok := arg.Value.(string); !ok {
				return fmt.Errorf("'%s' value should be of type string", arg.Name)
			}
		case DataInt64:
			if _, ok := arg.Value.(int64); !ok {
				return fmt.Errorf("'%s' value should be of type int64", arg.Name)
			}
		case DataUint64:
			if _, ok := arg.Value.(uint64); !ok {
				return fmt.Errorf("'%s' value should be of type uint64", arg.Name)
			}
		case DataFloat64:
			if _, ok := arg.Value.(float64); !ok {
				return fmt.Errorf("'%s' value should be of type float64", arg.Name)
			}
		case DataHash:
			if _, ok := arg.Value.(crypto.Hash); !ok {
				return fmt.Errorf("'%s' value should be of type Hash", arg.Name)
			}
		case DataAddress:
			if _, ok := arg.Value.(Address); !ok {
				return fmt.Errorf("'%s' value should be of type address", arg.Name)
			}
		case DataTime:
			if _, ok := arg.Value.(time.Time); !ok {
				return fmt.Errorf("'%s' value should be of type time", arg.Name)
			}

		default:
			return fmt.Errorf("unknown data type. Pls implement")
		}
	}
	return nil
}

// sort the arguments by their name
func (args *Arguments) Sort() {
	s := *args
	if len(*args) <= 1 {
		return
	}
	sort.Slice(s, func(i, j int) bool {
		return s[i].Name <= s[j].Name
	})

}

// some fields are already defined
// TODO we need to define ABI here to use names also, we have a name service

const RPC_DESTINATION_PORT = "D"  // mandatory,uint64,  used for ID of type uint64
const RPC_SOURCE_PORT = "S"       // mandatory,uint64, used for ID
const RPC_VALUE_TRANSFER = "V"    // uint64, this is representation and is only readable, value is never transferred
const RPC_COMMENT = "C"           // optional,string, used for display MSG to user
const RPC_EXPIRY = "E"            // optional,time used for Expiry for this service call
const RPC_REPLYBACK_ADDRESS = "R" // this is mandatory this is an address,otherwise how will otherside respond
const RPC_ASSET = "A"             // this is optional, a SCID to inform which asset we want to receive, by default DERO
//RPC will include own address so as the other enc can respond

const RPC_NEEDS_REPLYBACK_ADDRESS = "N" //optional, uint64

type argument_raw struct {
	Name     string          `json:"name"`     // string name must be atleast 1 byte
	DataType DataType        `json:"datatype"` // Type must one of the known data types
	Value    json.RawMessage `json:"value"`    //  delay parsing until we know the value should be as per type
}

func (a *Argument) UnmarshalJSON(b []byte) (err error) {
	var raw argument_raw
	if err = json.Unmarshal(b, &raw); err != nil {
		return err
	}
	a.Name = raw.Name
	a.DataType = raw.DataType
	switch raw.DataType {
	case DataString:
		var x string
		if err = json.Unmarshal(raw.Value, &x); err == nil {
			a.Value = x
			return
		}
	case DataInt64:
		var x int64
		if err = json.Unmarshal(raw.Value, &x); err == nil {
			a.Value = x
			return
		}
	case DataUint64:
		var x uint64
		if err = json.Unmarshal(raw.Value, &x); err == nil {
			a.Value = x
			return
		}
	case DataFloat64:
		var x float64
		if err = json.Unmarshal(raw.Value, &x); err == nil {
			a.Value = x
			return
		}
	case DataHash:
		var x crypto.Hash
		if err = json.Unmarshal(raw.Value, &x); err == nil {
			a.Value = x
			return
		}
	case DataAddress:
		var x Address
		if err = json.Unmarshal(raw.Value, &x); err == nil {
			a.Value = x
			return
		}
	case DataTime:
		var x time.Time
		if err = json.Unmarshal(raw.Value, &x); err == nil {
			a.Value = x
			return
		}
	default:
		return fmt.Errorf("unknown data type %s", raw.DataType)

	}

	return
}
