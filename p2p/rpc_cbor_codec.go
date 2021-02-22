package p2p

// this file implements CBOR codec to prevent from certain attacks
import "fmt"
import "io"
import "net"
import "net/rpc"
import "bufio"
import "encoding/binary"
import "github.com/fxamacker/cbor"

import "github.com/deroproject/derohe/config" // only used get constants such as max data per frame

// used to represent net/rpc structs
type Request struct {
	ServiceMethod string `cbor:"M"` // format: "Service.Method"
	Seq           uint64 `cbor:"S"` // sequence number chosen by client
}

type Response struct {
	ServiceMethod string `cbor:"M"` // echoes that of the Request
	Seq           uint64 `cbor:"S"` // echoes that of the request
	Error         string `cbor:"E"` // error, if any.
}

// reads our data, length prefix blocks
func Read_Data_Frame(r io.Reader, obj interface{}) error {
	var frame_length_buf [4]byte

	//connection.set_timeout()
	nbyte, err := io.ReadFull(r, frame_length_buf[:])
	if err != nil {
		return err
	}
	if nbyte != 4 {
		return fmt.Errorf("needed 4 bytes, but got %d bytes", nbyte)
	}

	//  time to ban
	frame_length := binary.LittleEndian.Uint32(frame_length_buf[:])
	if frame_length == 0 {
		return nil
	}
	// most probably memory DDOS attack, kill the connection
	if uint64(frame_length) > (5 * config.STARGATE_HE_MAX_BLOCK_SIZE) {
		return fmt.Errorf("Frame length is too big Expected %d Actual %d %s", 5*config.STARGATE_HE_MAX_BLOCK_SIZE, frame_length)
	}
	data_buf := make([]byte, frame_length)
	data_size, err := io.ReadFull(r, data_buf)
	if err != nil || data_size <= 0 || uint32(data_size) != frame_length {
		return fmt.Errorf("Could not read data size  read %d, frame length %d err %s", data_size, frame_length, err)
	}
	data_buf = data_buf[:frame_length]
	err = cbor.Unmarshal(data_buf, obj)

	//fmt.Printf("Read object %+v raw %s\n",obj, data_buf)
	return err
}

// reads our data, length prefix blocks
func Write_Data_Frame(w io.Writer, obj interface{}) error {
	var frame_length_buf [4]byte
	data_bytes, err := cbor.Marshal(obj)
	if err != nil {
		return err
	}
	binary.LittleEndian.PutUint32(frame_length_buf[:], uint32(len(data_bytes)))

	if _, err = w.Write(frame_length_buf[:]); err != nil {
		return err
	}
	_, err = w.Write(data_bytes[:])
	//fmt.Printf("Wrote object %+v raw %s\n",obj, data_bytes)
	return err
}

// ClientCodec implements the rpc.ClientCodec interface for generic golang objects.
type ClientCodec struct {
	r *bufio.Reader
	w io.WriteCloser
}

// ServerCodec implements the rpc.ServerCodec interface for generic protobufs.
type ServerCodec ClientCodec

// NewClientCodec returns a ClientCodec for communicating with the ServerCodec
// on the other end of the conn.
func NewCBORClientCodec(conn net.Conn) *ClientCodec {
	return &ClientCodec{bufio.NewReader(conn), conn}
}

// NewServerCodec returns a ServerCodec that communicates with the ClientCodec
// on the other end of the given conn.
func NewCBORServerCodec(conn net.Conn) *ServerCodec {
	return &ServerCodec{bufio.NewReader(conn), conn}
}

// WriteRequest writes the 4 byte length from the connection and encodes that many
// subsequent bytes into the given object.
func (c *ClientCodec) WriteRequest(req *rpc.Request, obj interface{}) error {
	// Write the header
	header := Request{ServiceMethod: req.ServiceMethod, Seq: req.Seq}
	if err := Write_Data_Frame(c.w, header); err != nil {
		return err
	}
	return Write_Data_Frame(c.w, obj)
}

// ReadResponseHeader reads a 4 byte length from the connection and decodes that many
// subsequent bytes into the given object, decodes it, and stores the fields
// in the given request.
func (c *ClientCodec) ReadResponseHeader(resp *rpc.Response) error {
	var header Response
	if err := Read_Data_Frame(c.r, &header); err != nil {
		return err
	}
	if header.ServiceMethod == "" {
		return fmt.Errorf("header missing method: %s", header)
	}
	resp.ServiceMethod = header.ServiceMethod
	resp.Seq = header.Seq
	resp.Error = header.Error

	return nil
}

// ReadResponseBody reads a 4 byte length from the connection and decodes that many
// subsequent bytes into the given object (which should be a pointer to a
// struct).
func (c *ClientCodec) ReadResponseBody(obj interface{}) error {
	if obj == nil {
		return nil
	}
	return Read_Data_Frame(c.r, obj)
}

// Close closes the underlying connection.
func (c *ClientCodec) Close() error {
	return c.w.Close()
}

// Close closes the underlying connection.
func (c *ServerCodec) Close() error {
	return c.w.Close()
}

// ReadRequestHeader reads the header (which is prefixed by a 4 byte lil endian length
// indicating its size) from the connection, decodes it, and stores the fields
// in the given request.
func (s *ServerCodec) ReadRequestHeader(req *rpc.Request) error {
	var header Request
	if err := Read_Data_Frame(s.r, &header); err != nil {
		return err
	}
	if header.ServiceMethod == "" {
		return fmt.Errorf("header missing method: %s", header)
	}
	req.ServiceMethod = header.ServiceMethod
	req.Seq = header.Seq
	return nil
}

// ReadRequestBody reads a 4 byte length from the connection and decodes that many
// subsequent bytes into the object
func (s *ServerCodec) ReadRequestBody(obj interface{}) error {
	if obj == nil {
		return nil
	}
	return Read_Data_Frame(s.r, obj)
}

// WriteResponse writes the appropriate header. If
// the response was invalid, the size of the body of the resp is reported as
// having size zero and is not sent.
func (s *ServerCodec) WriteResponse(resp *rpc.Response, obj interface{}) error {
	// Write the header
	header := Response{ServiceMethod: resp.ServiceMethod, Seq: resp.Seq, Error: resp.Error}

	if err := Write_Data_Frame(s.w, header); err != nil {
		return err
	}

	if resp.Error == "" { // only write response object if error is nil
		return Write_Data_Frame(s.w, obj)
	}

	return nil
}
