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

import "os"
import "path"

import "github.com/romana/rlog"
import "github.com/sirupsen/logrus"

type RLOG_HOOK struct { //  rlog HOOK
	dummy     string
	formatter *logrus.TextFormatter
}

var HOOK RLOG_HOOK

// setup default logging to current directory
func Init_rlog() {

	HOOK.formatter = new(logrus.TextFormatter)
	HOOK.formatter.DisableColors = true
	HOOK.formatter.DisableTimestamp = true

	if os.Getenv("RLOG_LOG_LEVEL") == "" {
		os.Setenv("RLOG_LOG_LEVEL", "DEBUG") // default logging in debug mode
	}

	if os.Getenv("RLOG_LOG_FILE") == "" {
		exename, _ := os.Executable()
		filename := path.Base(exename) + ".log"
		os.Setenv("RLOG_LOG_FILE", filename) // default log file name
	}

	if os.Getenv("RLOG_LOG_STREAM") == "" {
		os.Setenv("RLOG_LOG_STREAM", "NONE") // do not log to stdout/stderr
	}

	if os.Getenv("RLOG_CALLER_INFO") == "" {
		os.Setenv("RLOG_CALLER_INFO", "RLOG_CALLER_INFO") // log caller info
	}

	//os.Setenv("RLOG_TRACE_LEVEL", "10") //user can request tracing
	//os.Setenv("RLOG_LOG_LEVEL", "DEBUG")
	rlog.UpdateEnv()

}

// log logrus messages to rlog
func (hook *RLOG_HOOK) Fire(entry *logrus.Entry) error {
	msg, err := hook.formatter.Format(entry)
	if err == nil {
		rlog.Infof(string(msg)) // log to file
	}
	return nil
}

// Levels returns configured log levels., we log everything
func (hook *RLOG_HOOK) Levels() []logrus.Level {
	return logrus.AllLevels
}
