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

package p2p

//import "fmt"
import "time"
import "math/rand"

import "github.com/beevik/ntp"

import "github.com/deroproject/derohe/globals"

// these servers automatically rotate every hour as per documentation
// we also rotate them randomly
// TODO support ipv6
var timeservers = []string{ // facebook/google do leap smearing, so they should not be mixed here
	"0.pool.ntp.org",
	"1.pool.ntp.org",
	"2.pool.ntp.org",
	"3.pool.ntp.org",
	"ntp1.hetzner.de",
	"ntp2.hetzner.de",
	"ntp3.hetzner.de",
	"time.cloudflare.com", // anycast
	"ntp.se",              // anycast

}

// continusosly checks time for deviation if possible
// ToDo initial warning should NOT get hidden in messages
// TODO we need to spport interleaved NTP protocol, possibly over TCP
func time_check_routine() {

	const offset_count = 128
	var offsets [offset_count]time.Duration
	var offset_index int

	random := rand.New(globals.NewCryptoRandSource())
	timeinsync := false
	for {
		server := timeservers[random.Int()%len(timeservers)]

		if response, err := ntp.Query(server); err != nil {
			//logger.V(2).Error(err, "error while querying time", "server", server)
		} else if response.Validate() == nil {

			if response.ClockOffset.Seconds() > -.05 && response.ClockOffset.Seconds() < .05 {

			}
			offsets[offset_index] = response.ClockOffset
			offset_index = (offset_index + 1) % offset_count

			var avg_offset time.Duration
			var avg_count time.Duration
			for _, o := range offsets {
				if o != 0 {
					avg_offset += o
					avg_count++
				}
			}
			avg_offset = avg_offset / avg_count

			// if offset is small, do not trust ourselves but instead trust the system itself
			// we do not expect our resolution to be better than 50 ms
			if avg_offset.Milliseconds() < -50 || avg_offset.Milliseconds() > 50 {
				globals.ClockOffsetNTP = avg_offset
			} else {
				globals.ClockOffsetNTP = 0
			}
			globals.TimeIsInSyncNTP = true
			// if offset is more than 1 sec
			if response.ClockOffset.Seconds() > -1.0 && response.ClockOffset.Seconds() < 1.0 { // chrony can maintain upto 5 ms, ntps can maintain upto 10
				timeinsync = true
			} else {
				timeinsync = false
				logger.V(1).Error(nil, "Your system time deviation is more than 1 secs (%s)."+
					"\nYou may experience chain sync issues and/or other side-effects."+
					"\nIf you are mining, your blocks may get rejected."+
					"\nPlease sync your system using chrony/NTP software (default availble in all OS)."+
					"\n eg. ntpdate pool.ntp.org  (for linux/unix)", "offset", response.ClockOffset)
			}
		}

		if !timeinsync {
			time.Sleep(5 * time.Second)
		} else {
			time.Sleep(time.Duration((random.Intn(60) + 60)) * time.Second) // check every 60 + random(60) secs to avoid fingerprinting
		}
	}
}
