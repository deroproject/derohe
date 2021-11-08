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

import "fmt"
import "io"
import "os"
import "time"
import "bytes"
import "net"
import "net/url"
import "net/http"
import "path/filepath"
import "github.com/go-logr/logr"
import "github.com/VictoriaMetrics/metrics"

// these are exported by the daemon for various analysis
var Version string //this is later converted to metrics format

// we may need to expose various p2p stats, but currently they can be skipped
// this is measure by when we first see and when we see it again from a different peer
var Propagation_Block = metrics.NewHistogram(`block_propagation_duration_histogram_seconds`)
var Propagation_Transaction = metrics.NewHistogram(`transaction_propagation_duration_histogram_seconds`)
var Propagation_Chunk = metrics.NewHistogram(`chunk_propagation_duration_histogram_seconds`)

var startTime = time.Now()

var Set = metrics.NewSet() //all metrics are stored here

// this is used if an agent wants to scrap
func WritePrometheus(w http.ResponseWriter, req *http.Request) {
	writePrometheusMetrics(w)
}

func writePrometheusMetrics(w io.Writer) {
	metrics.WritePrometheus(w, true)
	metrics.WriteFDMetrics(w)
	Set.WritePrometheus(w)

	// Export start time and uptime in seconds
	fmt.Fprintf(w, "app_start_timestamp %d\n", startTime.Unix())
	fmt.Fprintf(w, "app_uptime_seconds %d\n", int(time.Since(startTime).Seconds()))
	fmt.Fprintf(w, "app_version{version=%q, short_version=%q} 1\n", Version, Version)

	usage := NewDiskUsage(".")
	fmt.Fprintf(w, "free_disk_space_bytes %d\n", usage.Available())
}

func Dump_metrics_data_directly(logger logr.Logger, specificnamei interface{}) {
	if os.Getenv("METRICS_SERVER") == "" { // daemon must have been started with this data
		return
	}

	metrics_url := os.Getenv("METRICS_SERVER")
	databuffer := bytes.NewBuffer(nil)

	var netTransport = &http.Transport{
		Dial: (&net.Dialer{
			Timeout: 5 * time.Second,
		}).Dial,
		TLSHandshakeTimeout: 5 * time.Second,
	}
	var netClient = &http.Client{
		Timeout:   time.Second * 5,
		Transport: netTransport,
	}

	u, err := url.Parse(metrics_url)
	if err != nil {
		logger.Error(err, "err parsing metrics url", "url", metrics_url)
		return
	}
	// remove any extra paths
	u.RawQuery = ""
	u.Fragment = ""
	u.Path = ""
	u.RawPath = ""
	u.RawFragment = ""

	metrics_url = u.String() + "/api/v1/import/prometheus"

	job_name := ""
	if hname, err := os.Hostname(); err == nil {
		job_name = hname
	}

	if binary_name, err := os.Executable(); err == nil {
		job_name += "_" + filepath.Base(binary_name)
	}

	if specificnamei != nil {
		if specificname := specificnamei.(string); specificname != "" {
			job_name += "_" + specificname
		}
	}

	metrics_url += "?extra_label=job=" + job_name
	metrics_url += "&extra_label=instance=" + fmt.Sprintf("%d", os.Getpid())

	logger.Info("metrics will be dispatched every 2 secs", "url", metrics_url)
	for {
		time.Sleep(2 * time.Second)
		databuffer.Reset()
		writePrometheusMetrics(databuffer)

		resp, err := netClient.Post(metrics_url, "application/test", databuffer)
		if err == nil {
			resp.Body.Close()
		} else {
			//fmt.Printf("err dispatching metrics to server '%s' err: '%s'\n",metrics_url,err)
		}
	}
}
