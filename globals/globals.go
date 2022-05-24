// Copyright 2017-2021 DERO Project. All rights reserved.
// Use of this source code in any form is governed by RESEARCH license.
// license can be found in the LICENSE file.
// GPG: 0F39 E425 8C65 3947 702A  8234 08B2 0360 A03A 9DE8
//
//
// THIS SOFTWARE IS PROVIDED BY THE COPYRIGHT HOLDERS AND CONTRIBUTORS "AS IS" AND ANY
// EXPRESS OR IMPLIED WARRANTIES, INCLUDING, BUT NOT LIMITED TO, THE IMPLIED WARRANTIES OF
// MERCHANTABILITY AND FITNESS FOR A PARTICULAR PURPOSE ARE DISCLAIMED. IN NO EVENT SHALL
// THE COPYRIGHT HOLDER OR CONTRIBUTORS BE LIABLE FOR ANY DIRECT, INDIRECT, INCIDENTAL,
// SPECIAL, EXEMPLARY, OR CONSEQUENTIAL DAMAGES (INCLUDING, BUT NOT LIMITED TO,
// PROCUREMENT OF SUBSTITUTE GOODS OR SERVICES; LOSS OF USE, DATA, OR PROFITS; OR BUSINESS
// INTERRUPTION) HOWEVER CAUSED AND ON ANY THEORY OF LIABILITY, WHETHER IN CONTRACT,
// STRICT LIABILITY, OR TORT (INCLUDING NEGLIGENCE OR OTHERWISE) ARISING IN ANY WAY OUT OF
// THE USE OF THIS SOFTWARE, EVEN IF ADVISED OF THE POSSIBILITY OF SUCH DAMAGE.

package globals

import "io"
import "os"
import "fmt"
import "time"
import "math"
import "net/url"
import "strings"
import "strconv"
import "math/big"
import "path/filepath"
import "runtime/debug"
import "golang.org/x/net/proxy"

import "go.uber.org/zap"
import "go.uber.org/zap/zapcore"
import "github.com/go-logr/logr"
import "github.com/go-logr/zapr"
import "github.com/robfig/cron/v3"

import "github.com/deroproject/derohe/config"
import "github.com/deroproject/derohe/rpc"

// all the the global variables used by the program are stored here
// since the entire logic is designed around a state machine driven by external events
// once the core starts nothing changes until there is a network state change

var Subsystem_Active uint32 // atomic counter to show how many subsystems are active
var Exit_In_Progress bool
var StartTime = time.Now()

// on init this variable is updated to setup global config in 1 go
var Config config.CHAIN_CONFIG = config.Mainnet // default is mainnnet

// global logger all components will use it with context
var Logger logr.Logger = logr.Discard() // default discard all logs

var ClockOffset time.Duration    // actual clock offset that is used by the daemon
var ClockOffsetNTP time.Duration // clockoffset in reference to ntp servers
var ClockOffsetP2P time.Duration // clockoffset in reference to p2p averging
var TimeIsInSync bool            // whether time is in sync, if yes we do not use any clock offset but still we keep calculating them
var TimeIsInSyncNTP bool

// get current time with clock offset applied
func Time() time.Time {
	if TimeIsInSync {
		return time.Now()
	}
	if TimeIsInSyncNTP {
		return time.Now().Add(ClockOffsetNTP)
	}
	return time.Now()
	//return time.Now().Add(ClockOffsetP2P) // this is the last effort
}

// skip p2p offset
func TimeSkipP2P() time.Time {
	if TimeIsInSync {
		return time.Now()
	}
	if TimeIsInSyncNTP {
		return time.Now().Add(ClockOffsetNTP)
	}
	return time.Now()
}

func GetOffset() time.Duration {
	return time.Now().Sub(Time())
}
func GetOffsetNTP() time.Duration {
	return ClockOffsetNTP
}
func GetOffsetP2P() time.Duration {
	return ClockOffsetP2P
}

var Cron = cron.New(cron.WithChain(
	cron.Recover(Logger), // or use cron.DefaultLogger
))

var Dialer proxy.Dialer = proxy.Direct // for proxy and direct connections
// all outgoing connections , including DNS requests must be made using this

// all program arguments are available here
var Arguments = map[string]interface{}{}

func InitNetwork() {
	Config = config.Mainnet                    // default is mainnnet
	if Arguments["--testnet"].(bool) == true { // setup testnet if requested
		Config = config.Testnet
	}

}

// these 2 global variables control all log levels
var Log_Level_Console = zap.NewAtomicLevelAt(zapcore.Level(0)) // default info level
var Log_Level_File = zap.NewAtomicLevelAt(zapcore.Level(-1))   // default debug level

// remove caller information from console
type removeCallerCore struct {
	zapcore.Core
}

func (c *removeCallerCore) Check(entry zapcore.Entry, ce *zapcore.CheckedEntry) *zapcore.CheckedEntry {
	if c.Core.Check(entry, nil) == nil {
		return ce
	}
	return ce.AddCore(entry, c)
}
func (c *removeCallerCore) Write(entry zapcore.Entry, fields []zapcore.Field) error {
	entry.Caller = zapcore.EntryCaller{}
	return c.Core.Write(entry, fields)
}
func (c *removeCallerCore) With(fields []zap.Field) zapcore.Core {
	return &removeCallerCore{c.Core.With(fields)}
}

func InitializeLog(console, logfile io.Writer) {

	if Arguments["--debug"] != nil && Arguments["--debug"].(bool) == true { // setup debug mode if requested
		Log_Level_Console = zap.NewAtomicLevelAt(zapcore.Level(-1))
	}

	if Arguments["--clog-level"] != nil { // setup log level if requested
		var log_level int8
		fmt.Sscan(Arguments["--clog-level"].(string), &log_level)
		if log_level < 0 {
			log_level = 0
		}
		if log_level > 127 {
			log_level = 127
		}
		Log_Level_Console = zap.NewAtomicLevelAt(zapcore.Level(0 - log_level))
	}

	if Arguments["--flog-level"] != nil { // setup log level if requested
		var log_level int8
		fmt.Sscan(Arguments["--flog-level"].(string), &log_level)
		if log_level < 0 {
			log_level = 0
		}
		if log_level > 127 {
			log_level = 127
		}
		Log_Level_File = zap.NewAtomicLevelAt(zapcore.Level(0 - log_level))
	}

	zf := zap.NewDevelopmentEncoderConfig()
	zc := zap.NewDevelopmentEncoderConfig()
	zc.EncodeLevel = zapcore.CapitalColorLevelEncoder
	zc.EncodeTime = zapcore.TimeEncoderOfLayout("02/01 15:04:05")

	file_encoder := zapcore.NewJSONEncoder(zf)
	console_encoder := zapcore.NewConsoleEncoder(zc)

	core_console := zapcore.NewCore(console_encoder, zapcore.AddSync(console), Log_Level_Console)
	removecore := &removeCallerCore{core_console}
	core := zapcore.NewTee(
		removecore,
		zapcore.NewCore(file_encoder, zapcore.AddSync(logfile), Log_Level_File),
	)

	zcore := zap.New(core, zap.AddCaller()) // add caller info to every record which is then trimmed from console

	Logger = zapr.NewLogger(zcore) // sets up global logger
	//Logger = zapr.NewLoggerWithOptions(zcore,zapr.LogInfoLevel("V")) // if you need verbosity levels

	// remember -1 is debug, 0 is info

}
func Initialize() {
	var err error

	InitNetwork()

	// choose  socks based proxy if user requested so
	if Arguments["--socks-proxy"] != nil {
		Logger.V(1).Info("Setting up proxy using ", "address", Arguments["--socks-proxy"].(string))
		//uri, err := url.Parse("socks5://127.0.0.1:9000") // "socks5://demo:demo@192.168.99.100:1080"
		uri, err := url.Parse("socks5://" + Arguments["--socks-proxy"].(string)) // "socks5://demo:demo@192.168.99.100:1080"
		if err != nil {
			Logger.Error(err, "Error parsing socks proxy:")
			os.Exit(-1)
		}

		Dialer, err = proxy.FromURL(uri, proxy.Direct)
		if err != nil {
			Logger.Error(err, "Error creating socks proxy", "address", Arguments["--socks-proxy"].(string))
		}
	}

	// lets create data directories
	err = os.MkdirAll(GetDataDirectory(), 0750)
	if err != nil {
		fmt.Printf("Error creating/accessing directory %s , err %s\n", GetDataDirectory(), err)
	}

}

// used to recover in case of panics
func Recover(level int) (err error) {
	if r := recover(); r != nil {
		err = fmt.Errorf("Recovered r:%+v stack %s", r, fmt.Sprintf("%s", string(debug.Stack())))
		Logger.V(level).Error(nil, "Recovered ", "error", r, "stack", fmt.Sprintf("%s", string(debug.Stack())))
	}
	return
}

// tells whether we are in mainnet mode
// if we are not mainnet, we are a testnet,
// we will only have a single mainnet ,( but we may have one or more testnets )
func IsMainnet() bool {
	return Config.Name == "mainnet"
}

// tells whether we are in simulator mode ( both mainnet and testnet coud be simulated)
func IsSimulator() bool {
	if Arguments["--simulator"] != nil && Arguments["--simulator"].(bool) == true {
		return true
	}
	return false
}

// return different directories for different networks ( mainly mainnet, testnet, simulation )
// this function is specifically for daemon
func GetDataDirectory() string {
	data_directory, err := os.Getwd()
	if err != nil {
		fmt.Printf("Error obtaining current directory, using temp dir err %s\n", err)
		data_directory = os.TempDir()
	}

	// if user provided an option, override default
	if Arguments["--data-dir"] != nil {
		data_directory = Arguments["--data-dir"].(string)
	}

	simulator := ""
	if IsSimulator() {
		simulator = "_simulator" // add _simulator
	}

	if IsMainnet() {
		return filepath.Join(data_directory, "mainnet"+simulator)
	}

	return filepath.Join(data_directory, "testnet"+simulator)
}

// never do any division operation on money due to floating point issues
// newbies, see type the next in python interpretor "3.33-3.13"
//
func FormatMoney(amount uint64) string {
	return FormatMoneyPrecision(amount, 5) // default is 5 precision after floating point
}

// 0
func FormatMoney0(amount uint64) string {
	return FormatMoneyPrecision(amount, 0)
}

//5 precision
func FormatMoney5(amount uint64) string {
	return FormatMoneyPrecision(amount, 5)
}

//8 precision
func FormatMoney8(amount uint64) string {
	return FormatMoneyPrecision(amount, 8)
}

// 12 precision
func FormatMoney12(amount uint64) string {
	return FormatMoneyPrecision(amount, 12) // default is 8 precision after floating point
}

// format money with specific precision
func FormatMoneyPrecision(amount uint64, precision int) string {
	hard_coded_decimals := new(big.Float).SetInt64(100000)
	float_amount, _, _ := big.ParseFloat(fmt.Sprintf("%d", amount), 10, 0, big.ToZero)
	result := new(big.Float)
	result.Quo(float_amount, hard_coded_decimals)
	return result.Text('f', precision) // 5 is display precision after floating point
}

// this will parse and validate an address, in reference to the current main/test mode
func ParseValidateAddress(str string) (addr *rpc.Address, err error) {
	addr, err = rpc.NewAddress(strings.TrimSpace(str))
	if err != nil {
		return
	}

	// check whether the domain is valid
	if !addr.IsDERONetwork() {
		err = fmt.Errorf("Invalid DERO address")
		return
	}

	if IsMainnet() != addr.IsMainnet() {
		if IsMainnet() {
			err = fmt.Errorf("Address belongs to DERO testnet and is invalid on current network")
		} else {
			err = fmt.Errorf("Address belongs to DERO mainnet and is invalid on current network")
		}
		return
	}

	return
}

// this will covert an amount in string form to atomic units
func ParseAmount(str string) (amount uint64, err error) {
	float_amount, base, err := big.ParseFloat(strings.TrimSpace(str), 10, 0, big.ToZero)

	if err != nil {
		err = fmt.Errorf("Amount could not be parsed err: %s", err)
		return
	}
	if base != 10 {
		err = fmt.Errorf("Amount should be in base 10 (0123456789)")
		return
	}
	if float_amount.Cmp(new(big.Float).Abs(float_amount)) != 0 { // number and abs(num) not equal means number is neg
		err = fmt.Errorf("Amount cannot be negative")
		return
	}

	// multiply by 5 zeroes
	hard_coded_decimals := new(big.Float).SetInt64(100000)
	float_amount.Mul(float_amount, hard_coded_decimals)

	/*if !float_amount.IsInt() {
	    err =  fmt.Errorf("Amount  is invalid %s ", float_amount.Text('f',0))
	    return
	}*/

	// convert amount to uint64
	//amount, _ = float_amount.Uint64() // sanity checks again
	amount, err = strconv.ParseUint(float_amount.Text('f', 0), 10, 64)
	if err != nil {
		err = fmt.Errorf("Amount  is invalid %s ", float_amount.Text('f', 0))
		return
	}
	//if amount == 0 {
	//	err = fmt.Errorf("0 cannot be transferred")
	//	return
	//}

	if amount == math.MaxUint64 {
		err = fmt.Errorf("Amount  is invalid")
		return
	}

	return // return the number
}

// gets a stack trace of all
func StackTrace(all bool) string {
	return string(debug.Stack())
}
