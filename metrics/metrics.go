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

// this file is the main metrics handler without any cyclic dependency on any other component

package metrics

import "net/http"
import  "github.com/VictoriaMetrics/metrics"

// these are exported by the daemon for various analysis


var Blockchain_tx_counter = metrics.NewCounter(`blockchain_tx_counter`)
var Mempool_tx_counter = metrics.NewCounter(`mempool_tx_counter`)
var Mempool_tx_count = metrics.NewCounter(`mempool_tx_count`) // its actually a gauge
var Block_size = metrics.NewHistogram(`block_size`)
var Block_tx = metrics.NewHistogram(`block_tx`)
var Block_processing_time = metrics.NewHistogram(`block_processing_time`)
var Transaction_size = metrics.NewHistogram(`transaction_size`)
var Transaction_ring_size = metrics.NewHistogram(`transaction_ring_size`)
var Transaction_outputs = metrics.NewHistogram("transaction_outputs") // a single tx will give to so many people

// we may need to expose various p2p stats, but currently they can be skipped

var Block_propagation = metrics.NewHistogram(`block_propagation`)
var Transaction_propagation = metrics.NewHistogram(`transaction_propagation`)


func WritePrometheus(w http.ResponseWriter, req *http.Request){
	 metrics.WritePrometheus(w, true)
}