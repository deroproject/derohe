package p2p

// this file implements time related code for syncing the unsyncronised servers
// yes, this project contains a local averaging distributed clock syncronisation algorithm
import "time"

// org = Origin Timestamp (client send time)
// rec = Receive Timestamp (server receive time)
// xmt = Transmit Timestamp (server reply time)
// dst = Destination Timestamp (client receive time)
func rtt(org, rec, xmt, dst time.Time) time.Duration {
	// round trip delay time rtt = (dst-org) - (xmt-rec)
	a := dst.Sub(org)
	b := xmt.Sub(rec)
	rtt := a - b
	if rtt < 0 {
		rtt = 0
	}
	return rtt
}

// all inputs are in micro secs,output is in nsec
func rtt_micro(org, rec, xmt, dst int64) time.Duration {
	// round trip delay time rtt = (dst-org) - (xmt-rec)
	a := dst - org
	b := xmt - rec
	rtt := a - b
	if rtt < 0 {
		rtt = 0
	}
	return time.Duration(rtt * 1000)
}

func offset(org, rec, xmt, dst time.Time) time.Duration {
	// local clock offset = ((rec-org) + (xmt-dst)) / 2
	a := rec.Sub(org)
	b := xmt.Sub(dst)
	return (a + b) / time.Duration(2)
}

// all inputs are in micro secs, output is in nsec
func offset_micro(org, rec, xmt, dst int64) time.Duration {
	// local clock offset = ((rec-org) + (xmt-dst)) / 2
	a := rec - org
	b := xmt - dst
	return time.Duration((a+b)*1000) / time.Duration(2)
}
