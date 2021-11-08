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

package main

// this file implements the explorer for DERO blockchain
// this needs only RPC access
// NOTE: Only use data exported from within the RPC interface, do direct use of exported variables  fom packages
// NOTE: we can use structs defined within the RPCserver package

// TODO: error handling is non-existant ( as this was built up in hrs ). Add proper error handling
//

import "time"
import "fmt"
import "os"
import "runtime"

import "github.com/docopt/docopt-go"
import "github.com/go-logr/logr"

import "github.com/deroproject/derohe/cmd/explorer/explorerlib"
import "github.com/deroproject/derohe/globals"

var command_line string = `dero_explorer
DERO HE Explorer: A secure, private blockchain with smart-contracts

Usage:
  dero_explorer [--help] [--version] [--debug] [--daemon-address=<127.0.0.1:18091>] [--http-address=<0.0.0.0:8080>] 
  dero_explorer -h | --help
  dero_explorer --version

Options:
  -h --help     Show this screen.
  --version     Show version.
  --debug       Debug mode enabled, print log messages
  --daemon-address=<127.0.0.1:10102>  connect to this daemon port as client
  --http-address=<0.0.0.0:8080>    explorer listens on this port to serve user requests`

var logger logr.Logger

func main() {
	var err error
	globals.Arguments, err = docopt.Parse(command_line, nil, true, "DERO Explorer : work in progress", false)

	if err != nil {
		fmt.Printf("Error while parsing options err: %s\n", err)
		return
	}

	exename, _ := os.Executable()
	f, err := os.Create(exename + ".log")
	if err != nil {
		fmt.Printf("Error while opening log file err: %s filename %s\n", err, exename+".log")
		return
	}
	globals.InitializeLog(os.Stdout, f)

	logger = globals.Logger.WithName("explorer")

	logger.Info("DERO HE explorer :  It is an alpha version, use it for testing/evaluations purpose only.")
	logger.Info("Copyright 2017-2021 DERO Project. All rights reserved.")
	logger.Info("", "OS", runtime.GOOS, "ARCH", runtime.GOARCH, "GOMAXPROCS", runtime.GOMAXPROCS(0))
	//logger.Info("","Version", config.Version.String())

	logger.V(1).Info("", "Arguments", globals.Arguments)

	endpoint := "127.0.0.1:8080"
	if globals.Arguments["--daemon-address"] != nil {
		endpoint = globals.Arguments["--daemon-address"].(string)
	}

	listen_address := "0.0.0.0:8081"
	if globals.Arguments["--http-address"] != nil {
		listen_address = globals.Arguments["--http-address"].(string)
	}

	if err = explorerlib.StartServer(logger, endpoint, listen_address); err == nil {
		for {
			time.Sleep(time.Second)
		}
	}
}
