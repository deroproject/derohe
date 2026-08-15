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

package config

// all global configuration variables are picked from here

// some seed nodes for mainnet (these seed node are not compliant with earlier protocols)
// only version 2
// Evaluated 2026-04-22 via multi-region KCP handshake test.
// Gaps: Africa, Australia/Oceania, NA-West-Coast, EU-East.
var Mainnet_seed_nodes = []string{
	// EU West — France
	"82.65.143.182:11011",
	// EU West — Germany (non-standard port)
	"85.214.253.170:58686",
	// EU West — Netherlands
	"38.180.116.63:11011",
	// EU West — UK (alternate port)
	"213.171.208.37:18089",
	// South America — Argentina
	"190.194.227.11:11011",
	// North America East — Canada
	"51.222.86.51:11011",
	// North America Central — USA (non-standard port)
	"209.145.59.4:50404",
	// Asia — Vietnam
	"116.111.112.188:11011",
}

// some seed node for testnet
var Testnet_seed_nodes = []string{
	"69.30.234.163:40401",
	"testnet.derofoundation.co:40401",
}
