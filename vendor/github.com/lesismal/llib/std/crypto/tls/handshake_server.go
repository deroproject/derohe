// Copyright 2009 The Go Authors. All rights reserved.
// Use of this source code is governed by a BSD-style
// license that can be found in the LICENSE file.

package tls

import (
	"crypto"
	"crypto/ecdsa"
	"crypto/ed25519"
	"crypto/rsa"
	"crypto/subtle"
	"crypto/x509"
	"errors"
	"fmt"
	"hash"
	"io"
	"sync/atomic"
	"time"
)

const (
	stateServerHandshakeReadClientHello uint32 = 1

	// TLS 1.3
	stateServerHandshake13ProcessClientHello    uint32 = 101
	stateServerHandshake13CheckForResumption    uint32 = 102
	stateServerHandshake13PickCertificate       uint32 = 103
	stateServerHandshake13SendServerParameters  uint32 = 104
	stateServerHandshake13SendServerCertificate uint32 = 105
	stateServerHandshake13SendServerFinished    uint32 = 106
	stateServerHandshake13ReadClientCertificate uint32 = 107
	stateServerHandshake13ReadClientFinished    uint32 = 108
	stateServerHandshake13HandshakeDone         uint32 = 109

	// below TLS 1.3
	stateServerHandshakeProcessClientHello uint32 = 201
	stateServerHandshakeCheckForResumption uint32 = 202

	stateServerHandshakeDoResumeHandshake uint32 = 203
	stateServerHandshakeEstablishKeys     uint32 = 204
	stateServerHandshakeSendSessionTicket uint32 = 205
	stateServerHandshakeSendFinished      uint32 = 206

	stateServerHandshakePickCipherSuite2                       uint32 = 208
	stateServerHandshakeDoFullHandshake2                       uint32 = 209
	stateServerHandshakeDoFullHandshake2ReadHandshake1         uint32 = 210
	stateServerHandshakeDoFullHandshake2HandleCertificateMsg   uint32 = 211
	stateServerHandshakeDoFullHandshake2ReadHandshake2         uint32 = 212
	stateServerHandshakeDoFullHandshake2HandleVerifyConnection uint32 = 213
	stateServerHandshakeDoFullHandshake2ReadHandshake3         uint32 = 214
	stateServerHandshakeEstablishKeys2                         uint32 = 215

	stateServerHandshakeReadFinishedReadChangeCipherSpec uint32 = 216
	stateServerHandshakeReadFinishedDone                 uint32 = 217

	stateServerHandshakeSendSessionTicket2 uint32 = 218
	stateServerHandshakeSendFinished2      uint32 = 219

	stateServerHandshakeHandshakeDone uint32 = 220
)

// serverHandshakeState contains details of a server handshake in progress.
// It's discarded once the handshake has completed.
type serverHandshakeState struct {
	c  *Conn
	ok bool
	// msg          interface{}
	ka           keyAgreement
	clientHello  *clientHelloMsg
	hello        *serverHelloMsg
	suite        *cipherSuite
	ecdheOk      bool
	ecSignOk     bool
	rsaDecryptOk bool
	rsaSignOk    bool
	sessionState *sessionState
	finishedHash finishedHash
	masterSecret []byte
	cert         *Certificate

	err error
}

// serverHandshake performs a TLS handshake as a server.
func (c *Conn) serverHandshake() error {
	var err error

	if c.handshakeStatusAsync < stateServerHandshakeReadClientHello {
		c.clientHello, err = c.readClientHello()
		if err != nil {
			return err
		}
		c.handshakeStatusAsync = stateServerHandshakeReadClientHello
	}
	if c.vers == VersionTLS13 {
		hs := c.hs13
		if hs == nil {
			hs = &serverHandshakeStateTLS13{
				c:           c,
				clientHello: c.clientHello,
			}
			c.hs13 = hs
		}
		return hs.handshake()
	}

	hs := c.hs
	if hs == nil {
		hs = &serverHandshakeState{
			c:           c,
			clientHello: c.clientHello,
		}
		c.hs = hs
	}
	return hs.handshake()
}

func (hs *serverHandshakeState) handshake() error {
	c := hs.c
	if c.handshakeStatusAsync >= stateServerHandshakeHandshakeDone {
		return nil
	}
	if hs.err != nil && hs.err != errDataNotEnough {
		return hs.err
	}

	if err := hs.processClientHello(); err != nil {
		hs.err = err
		return err
	}

	// For an overview of TLS handshaking, see RFC 5246, Section 7.3.
	if hs.checkForResumption() {
		// The client has included a session ticket and so we do an abbreviated handshake.
		c.didResume = true
		if err := hs.doResumeHandshake(); err != nil {
			hs.err = err
			return err
		}
		if err := hs.establishKeys(); err != nil {
			hs.err = err
			return err
		}
		c.buffering = true
		if err := hs.sendSessionTicket(); err != nil {
			hs.err = err
			return err
		}
		if err := hs.sendFinished(c.serverFinished[:]); err != nil {
			hs.err = err
			return err
		}
		if _, err := c.flush(); err != nil {
			hs.err = err
			return err
		}
		c.clientFinishedIsFirst = false
		if err := hs.readFinished(nil); err != nil {
			hs.err = err
			return err
		}
	} else {
		// The client didn't include a session ticket, or it wasn't
		// valid so we do a full handshake.
		if err := hs.pickCipherSuite(); err != nil {
			hs.err = err
			return err
		}
		if err := hs.doFullHandshake(); err != nil {
			hs.err = err
			if err != errDataNotEnough {
			}
			return err
		}
		if err := hs.establishKeys(); err != nil {
			hs.err = err
			return err
		}
		if err := hs.readFinished(c.clientFinished[:]); err != nil {
			hs.err = err
			if err != errDataNotEnough {
			}
			return err
		}
		c.clientFinishedIsFirst = true
		c.buffering = true
		if err := hs.sendSessionTicket2(); err != nil {
			hs.err = err
			return err
		}
		if err := hs.sendFinished2(nil); err != nil {
			hs.err = err
			return err
		}
		if _, err := c.flush(); err != nil {
			hs.err = err
			return err
		}
	}

	c.ekm = ekmFromMasterSecret(c.vers, hs.suite, hs.masterSecret, hs.clientHello.random, hs.hello.random)
	atomic.StoreUint32(&c.handshakeStatus, 1)

	c.handshakeStatusAsync = stateServerHandshakeHandshakeDone

	return nil
}

// readClientHello reads a ClientHello message and selects the protocol version.
func (c *Conn) readClientHello() (*clientHelloMsg, error) {
	msg, err := c.readHandshake()
	if err != nil {
		return nil, err
	}
	clientHello, ok := msg.(*clientHelloMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		return nil, unexpectedMessageError(clientHello, msg)
	}

	var configForClient *Config
	originalConfig := c.config
	if c.config.GetConfigForClient != nil {
		chi := clientHelloInfo(c, clientHello)
		if configForClient, err = c.config.GetConfigForClient(chi); err != nil {
			c.sendAlert(alertInternalError)
			return nil, err
		} else if configForClient != nil {
			c.config = configForClient
		}
	}
	c.ticketKeys = originalConfig.ticketKeys(configForClient)

	clientVersions := clientHello.supportedVersions
	if len(clientHello.supportedVersions) == 0 {
		clientVersions = supportedVersionsFromMax(clientHello.vers)
	}
	c.vers, ok = c.config.mutualVersion(clientVersions)
	if !ok {
		c.sendAlert(alertProtocolVersion)
		return nil, fmt.Errorf("tls: client offered only unsupported versions: %x", clientVersions)
	}
	c.haveVers = true
	c.in.version = c.vers
	c.out.version = c.vers

	return clientHello, nil
}

func (hs *serverHandshakeState) processClientHello() error {
	c := hs.c

	if c.handshakeStatusAsync >= stateServerHandshakeProcessClientHello {
		return nil
	}
	c.handshakeStatusAsync = stateServerHandshakeProcessClientHello

	hs.hello = new(serverHelloMsg)
	hs.hello.vers = c.vers

	foundCompression := false
	// We only support null compression, so check that the client offered it.
	for _, compression := range hs.clientHello.compressionMethods {
		if compression == compressionNone {
			foundCompression = true
			break
		}
	}

	if !foundCompression {
		c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: client does not support uncompressed connections")
	}

	hs.hello.random = make([]byte, 32)
	serverRandom := hs.hello.random
	// Downgrade protection canaries. See RFC 8446, Section 4.1.3.
	maxVers := c.config.maxSupportedVersion()
	if maxVers >= VersionTLS12 && c.vers < maxVers || testingOnlyForceDowngradeCanary {
		if c.vers == VersionTLS12 {
			copy(serverRandom[24:], downgradeCanaryTLS12)
		} else {
			copy(serverRandom[24:], downgradeCanaryTLS11)
		}
		serverRandom = serverRandom[:24]
	}
	_, err := io.ReadFull(c.config.rand(), serverRandom)
	if err != nil {
		c.sendAlert(alertInternalError)
		return err
	}

	if len(hs.clientHello.secureRenegotiation) != 0 {
		c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: initial handshake had non-empty renegotiation extension")
	}

	hs.hello.secureRenegotiationSupported = hs.clientHello.secureRenegotiationSupported
	hs.hello.compressionMethod = compressionNone
	if len(hs.clientHello.serverName) > 0 {
		c.serverName = hs.clientHello.serverName
	}

	if len(hs.clientHello.alpnProtocols) > 0 {
		if selectedProto := mutualProtocol(hs.clientHello.alpnProtocols, c.config.NextProtos); selectedProto != "" {
			hs.hello.alpnProtocol = selectedProto
			c.clientProtocol = selectedProto
		}
	}

	hs.cert, err = c.config.getCertificate(clientHelloInfo(c, hs.clientHello))
	if err != nil {
		if err == errNoCertificates {
			c.sendAlert(alertUnrecognizedName)
		} else {
			c.sendAlert(alertInternalError)
		}
		return err
	}
	if hs.clientHello.scts {
		hs.hello.scts = hs.cert.SignedCertificateTimestamps
	}

	hs.ecdheOk = supportsECDHE(c.config, hs.clientHello.supportedCurves, hs.clientHello.supportedPoints)

	if hs.ecdheOk {
		// Although omitting the ec_point_formats extension is permitted, some
		// old OpenSSL version will refuse to handshake if not present.
		//
		// Per RFC 4492, section 5.1.2, implementations MUST support the
		// uncompressed point format. See golang.org/issue/31943.
		hs.hello.supportedPoints = []uint8{pointFormatUncompressed}
	}

	if priv, ok := hs.cert.PrivateKey.(crypto.Signer); ok {
		switch priv.Public().(type) {
		case *ecdsa.PublicKey:
			hs.ecSignOk = true
		case ed25519.PublicKey:
			hs.ecSignOk = true
		case *rsa.PublicKey:
			hs.rsaSignOk = true
		default:
			c.sendAlert(alertInternalError)
			return fmt.Errorf("tls: unsupported signing key type (%T)", priv.Public())
		}
	}
	if priv, ok := hs.cert.PrivateKey.(crypto.Decrypter); ok {
		switch priv.Public().(type) {
		case *rsa.PublicKey:
			hs.rsaDecryptOk = true
		default:
			c.sendAlert(alertInternalError)
			return fmt.Errorf("tls: unsupported decryption key type (%T)", priv.Public())
		}
	}

	return nil
}

// supportsECDHE returns whether ECDHE key exchanges can be used with this
// pre-TLS 1.3 client.
func supportsECDHE(c *Config, supportedCurves []CurveID, supportedPoints []uint8) bool {
	supportsCurve := false
	for _, curve := range supportedCurves {
		if c.supportsCurve(curve) {
			supportsCurve = true
			break
		}
	}

	supportsPointFormat := false
	for _, pointFormat := range supportedPoints {
		if pointFormat == pointFormatUncompressed {
			supportsPointFormat = true
			break
		}
	}

	return supportsCurve && supportsPointFormat
}

func (hs *serverHandshakeState) pickCipherSuite() error {
	c := hs.c

	if c.handshakeStatusAsync >= stateServerHandshakePickCipherSuite2 {
		return nil
	}
	c.handshakeStatusAsync = stateServerHandshakePickCipherSuite2

	var preferenceList, supportedList []uint16
	if c.config.PreferServerCipherSuites {
		preferenceList = c.config.cipherSuites()
		supportedList = hs.clientHello.cipherSuites

		// If the client does not seem to have hardware support for AES-GCM,
		// and the application did not specify a cipher suite preference order,
		// prefer other AEAD ciphers even if we prioritized AES-GCM ciphers
		// by default.
		if c.config.CipherSuites == nil && !aesgcmPreferred(hs.clientHello.cipherSuites) {
			preferenceList = deprioritizeAES(preferenceList)
		}
	} else {
		preferenceList = hs.clientHello.cipherSuites
		supportedList = c.config.cipherSuites()

		// If we don't have hardware support for AES-GCM, prefer other AEAD
		// ciphers even if the client prioritized AES-GCM.
		if !hasAESGCMHardwareSupport {
			preferenceList = deprioritizeAES(preferenceList)
		}
	}

	hs.suite = selectCipherSuite(preferenceList, supportedList, hs.cipherSuiteOk)
	if hs.suite == nil {
		c.sendAlert(alertHandshakeFailure)
		return errors.New("tls: no cipher suite supported by both client and server")
	}
	c.cipherSuite = hs.suite.id

	for _, id := range hs.clientHello.cipherSuites {
		if id == TLS_FALLBACK_SCSV {
			// The client is doing a fallback connection. See RFC 7507.
			if hs.clientHello.vers < c.config.maxSupportedVersion() {
				c.sendAlert(alertInappropriateFallback)
				return errors.New("tls: client using inappropriate protocol fallback")
			}
			break
		}
	}

	return nil
}

func (hs *serverHandshakeState) cipherSuiteOk(c *cipherSuite) bool {
	if c.flags&suiteECDHE != 0 {
		if !hs.ecdheOk {
			return false
		}
		if c.flags&suiteECSign != 0 {
			if !hs.ecSignOk {
				return false
			}
		} else if !hs.rsaSignOk {
			return false
		}
	} else if !hs.rsaDecryptOk {
		return false
	}
	if hs.c.vers < VersionTLS12 && c.flags&suiteTLS12 != 0 {
		return false
	}
	return true
}

// checkForResumption reports whether we should perform resumption on this connection.
func (hs *serverHandshakeState) checkForResumption() bool {
	c := hs.c

	if c.handshakeStatusAsync >= stateServerHandshakeCheckForResumption {
		return hs.ok
	}
	c.handshakeStatusAsync = stateServerHandshakeCheckForResumption

	if c.config.SessionTicketsDisabled {
		hs.ok = false
		return false
	}

	plaintext, usedOldKey := c.decryptTicket(hs.clientHello.sessionTicket)
	if plaintext == nil {
		hs.ok = false
		return false
	}
	hs.sessionState = &sessionState{usedOldKey: usedOldKey}
	ok := hs.sessionState.unmarshal(plaintext)
	if !ok {
		hs.ok = false
		return false
	}

	createdAt := time.Unix(int64(hs.sessionState.createdAt), 0)
	if c.config.time().Sub(createdAt) > maxSessionTicketLifetime {
		hs.ok = false
		return false
	}

	// Never resume a session for a different TLS version.
	if c.vers != hs.sessionState.vers {
		hs.ok = false
		return false
	}

	cipherSuiteOk := false
	// Check that the client is still offering the ciphersuite in the session.
	for _, id := range hs.clientHello.cipherSuites {
		if id == hs.sessionState.cipherSuite {
			cipherSuiteOk = true
			break
		}
	}
	if !cipherSuiteOk {
		hs.ok = false
		return false
	}

	// Check that we also support the ciphersuite from the session.
	hs.suite = selectCipherSuite([]uint16{hs.sessionState.cipherSuite},
		c.config.cipherSuites(), hs.cipherSuiteOk)
	if hs.suite == nil {
		hs.ok = false
		return false
	}

	sessionHasClientCerts := len(hs.sessionState.certificates) != 0
	needClientCerts := requiresClientCert(c.config.ClientAuth)
	if needClientCerts && !sessionHasClientCerts {
		hs.ok = false
		return false
	}
	if sessionHasClientCerts && c.config.ClientAuth == NoClientCert {
		hs.ok = false
		return false
	}
	hs.ok = true
	return true
}

func (hs *serverHandshakeState) doResumeHandshake() error {
	c := hs.c

	if c.handshakeStatusAsync >= stateServerHandshakeDoResumeHandshake {
		return nil
	}
	c.handshakeStatusAsync = stateServerHandshakeDoResumeHandshake

	hs.hello.cipherSuite = hs.suite.id
	c.cipherSuite = hs.suite.id
	// We echo the client's session ID in the ServerHello to let it know
	// that we're doing a resumption.
	hs.hello.sessionId = hs.clientHello.sessionId
	hs.hello.ticketSupported = hs.sessionState.usedOldKey
	hs.finishedHash = newFinishedHash(c.vers, hs.suite)
	hs.finishedHash.discardHandshakeBuffer()
	hs.finishedHash.Write(hs.clientHello.marshal())
	hs.finishedHash.Write(hs.hello.marshal())
	if _, err := c.writeRecord(recordTypeHandshake, hs.hello.marshal()); err != nil {
		return err
	}

	if err := c.processCertsFromClient(Certificate{
		Certificate: hs.sessionState.certificates,
	}); err != nil {
		return err
	}

	if c.config.VerifyConnection != nil {
		if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
			c.sendAlert(alertBadCertificate)
			return err
		}
	}

	hs.masterSecret = hs.sessionState.masterSecret

	return nil
}

func (hs *serverHandshakeState) doFullHandshake() error {
	c := hs.c

	var err error
	var msg interface{}
	var pub crypto.PublicKey // public key for client auth, if any
	var certReq *certificateRequestMsg
	if c.handshakeStatusAsync < stateServerHandshakeDoFullHandshake2 {
		c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2

		if hs.clientHello.ocspStapling && len(hs.cert.OCSPStaple) > 0 {
			hs.hello.ocspStapling = true
		}

		hs.hello.ticketSupported = hs.clientHello.ticketSupported && !c.config.SessionTicketsDisabled
		hs.hello.cipherSuite = hs.suite.id

		hs.finishedHash = newFinishedHash(hs.c.vers, hs.suite)
		if c.config.ClientAuth == NoClientCert {
			// No need to keep a full record of the handshake if client
			// certificates won't be used.
			hs.finishedHash.discardHandshakeBuffer()
		}

		c.buffering = true
		hs.finishedHash.Write(hs.clientHello.marshal())
		hs.finishedHash.Write(hs.hello.marshal())
		if _, err := c.writeRecord(recordTypeHandshake, hs.hello.marshal()); err != nil {
			return err
		}

		certMsg := new(certificateMsg)
		certMsg.certificates = hs.cert.Certificate
		hs.finishedHash.Write(certMsg.marshal())
		if _, err := c.writeRecord(recordTypeHandshake, certMsg.marshal()); err != nil {
			return err
		}

		if hs.hello.ocspStapling {
			certStatus := new(certificateStatusMsg)
			certStatus.response = hs.cert.OCSPStaple
			hs.finishedHash.Write(certStatus.marshal())
			if _, err := c.writeRecord(recordTypeHandshake, certStatus.marshal()); err != nil {
				return err
			}
		}

		hs.ka = hs.suite.ka(c.vers)
		skx, err := hs.ka.generateServerKeyExchange(c.config, hs.cert, hs.clientHello, hs.hello)
		if err != nil {
			c.sendAlert(alertHandshakeFailure)
			return err
		}
		if skx != nil {
			hs.finishedHash.Write(skx.marshal())
			if _, err := c.writeRecord(recordTypeHandshake, skx.marshal()); err != nil {
				return err
			}
		}

		if c.config.ClientAuth >= RequestClientCert {
			// Request a client certificate
			certReq = new(certificateRequestMsg)
			certReq.certificateTypes = []byte{
				byte(certTypeRSASign),
				byte(certTypeECDSASign),
			}
			if c.vers >= VersionTLS12 {
				certReq.hasSignatureAlgorithm = true
				certReq.supportedSignatureAlgorithms = supportedSignatureAlgorithms
			}

			// An empty list of certificateAuthorities signals to
			// the client that it may send any certificate in response
			// to our request. When we know the CAs we trust, then
			// we can send them down, so that the client can choose
			// an appropriate certificate to give to us.
			if c.config.ClientCAs != nil {
				certReq.certificateAuthorities = c.config.ClientCAs.Subjects()
			}
			hs.finishedHash.Write(certReq.marshal())
			if _, err := c.writeRecord(recordTypeHandshake, certReq.marshal()); err != nil {
				return err
			}
		}

		helloDone := new(serverHelloDoneMsg)
		hs.finishedHash.Write(helloDone.marshal())
		if _, err := c.writeRecord(recordTypeHandshake, helloDone.marshal()); err != nil {
			return err
		}

		if _, err := c.flush(); err != nil {
			return err
		}
	}

	if c.handshakeStatusAsync < stateServerHandshakeDoFullHandshake2ReadHandshake1 {
		msg, err = c.readHandshake()
		if err != nil {
			if err != errDataNotEnough {
				c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake1
			}
			return err
		}
		c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake1
	}

	// If we requested a client certificate, then the client must send a
	// certificate message, even if it's empty.

	if c.config.ClientAuth >= RequestClientCert {
		if c.handshakeStatusAsync < stateServerHandshakeDoFullHandshake2HandleCertificateMsg {
			c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2HandleCertificateMsg

			certMsg, ok := msg.(*certificateMsg)
			if !ok {
				c.sendAlert(alertUnexpectedMessage)
				return unexpectedMessageError(certMsg, msg)
			}
			hs.finishedHash.Write(certMsg.marshal())

			if err := c.processCertsFromClient(Certificate{
				Certificate: certMsg.certificates,
			}); err != nil {
				return err
			}
			if len(certMsg.certificates) != 0 {
				pub = c.peerCertificates[0].PublicKey
			}
		}
		if c.handshakeStatusAsync < stateServerHandshakeDoFullHandshake2ReadHandshake2 {
			msg, err = c.readHandshake()
			if err != nil {
				if err != errDataNotEnough {
					c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake2
				}
				return err
			}
			c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake2

		}
	}

	if c.handshakeStatusAsync < stateServerHandshakeDoFullHandshake2HandleVerifyConnection {
		c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2HandleVerifyConnection
		if c.config.VerifyConnection != nil {
			if err := c.config.VerifyConnection(c.connectionStateLocked()); err != nil {
				c.sendAlert(alertBadCertificate)
				return err
			}
		}

		// Get client key exchange
		ckx, ok := msg.(*clientKeyExchangeMsg)
		if !ok {
			c.sendAlert(alertUnexpectedMessage)
			return unexpectedMessageError(ckx, msg)
		}
		hs.finishedHash.Write(ckx.marshal())

		preMasterSecret, err := hs.ka.processClientKeyExchange(c.config, hs.cert, ckx, c.vers)
		if err != nil {
			c.sendAlert(alertHandshakeFailure)
			return err
		}
		hs.masterSecret = masterFromPreMasterSecret(c.vers, hs.suite, preMasterSecret, hs.clientHello.random, hs.hello.random)
		if err := c.config.writeKeyLog(keyLogLabelTLS12, hs.clientHello.random, hs.masterSecret); err != nil {
			c.sendAlert(alertInternalError)
			return err
		}

	}

	if c.handshakeStatusAsync >= stateServerHandshakeDoFullHandshake2ReadHandshake3 {
		return nil
	}
	// If we received a client cert in response to our certificate request message,
	// the client will send us a certificateVerifyMsg immediately after the
	// clientKeyExchangeMsg. This message is a digest of all preceding
	// handshake-layer messages that is signed using the private key corresponding
	// to the client's certificate. This allows us to verify that the client is in
	// possession of the private key of the certificate.
	if len(c.peerCertificates) > 0 {
		msg, err := c.readHandshake()
		if err != nil {
			if err != errDataNotEnough {
				c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake3
			}
			return err
		}
		certVerify, ok := msg.(*certificateVerifyMsg)
		if !ok {
			c.sendAlert(alertUnexpectedMessage)
			c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake3
			return unexpectedMessageError(certVerify, msg)
		}

		var sigType uint8
		var sigHash crypto.Hash
		if c.vers >= VersionTLS12 {
			if !isSupportedSignatureAlgorithm(certVerify.signatureAlgorithm, certReq.supportedSignatureAlgorithms) {
				c.sendAlert(alertIllegalParameter)
				c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake3
				return errors.New("tls: client certificate used with invalid signature algorithm")
			}
			sigType, sigHash, err = typeAndHashFromSignatureScheme(certVerify.signatureAlgorithm)
			if err != nil {
				c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake3
				return c.sendAlert(alertInternalError)
			}
		} else {
			sigType, sigHash, err = legacyTypeAndHashFromPublicKey(pub)
			if err != nil {
				c.sendAlert(alertIllegalParameter)
				c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake3
				return err
			}
		}

		signed := hs.finishedHash.hashForClientCertificate(sigType, sigHash, hs.masterSecret)
		if err := verifyHandshakeSignature(sigType, pub, sigHash, signed, certVerify.signature); err != nil {
			c.sendAlert(alertDecryptError)
			c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake3
			return errors.New("tls: invalid signature by the client certificate: " + err.Error())
		}

		hs.finishedHash.Write(certVerify.marshal())
	}

	hs.finishedHash.discardHandshakeBuffer()

	c.handshakeStatusAsync = stateServerHandshakeDoFullHandshake2ReadHandshake3

	return nil
}

func (hs *serverHandshakeState) establishKeys() error {
	c := hs.c
	if c.handshakeStatusAsync >= stateServerHandshakeEstablishKeys2 {
		return nil
	}
	c.handshakeStatusAsync = stateServerHandshakeEstablishKeys2

	clientMAC, serverMAC, clientKey, serverKey, clientIV, serverIV :=
		keysFromMasterSecret(c.vers, hs.suite, hs.masterSecret, hs.clientHello.random, hs.hello.random, hs.suite.macLen, hs.suite.keyLen, hs.suite.ivLen)

	var clientCipher, serverCipher interface{}
	var clientHash, serverHash hash.Hash

	if hs.suite.aead == nil {
		clientCipher = hs.suite.cipher(clientKey, clientIV, true /* for reading */)
		clientHash = hs.suite.mac(clientMAC)
		serverCipher = hs.suite.cipher(serverKey, serverIV, false /* not for reading */)
		serverHash = hs.suite.mac(serverMAC)
	} else {
		clientCipher = hs.suite.aead(clientKey, clientIV)
		serverCipher = hs.suite.aead(serverKey, serverIV)
	}

	c.in.prepareCipherSpec(c.vers, clientCipher, clientHash)
	c.out.prepareCipherSpec(c.vers, serverCipher, serverHash)
	return nil
}

func (hs *serverHandshakeState) readFinished(out []byte) error {
	c := hs.c

	if c.handshakeStatusAsync < stateServerHandshakeReadFinishedReadChangeCipherSpec {
		if err := c.readChangeCipherSpec(); err != nil {
			if err != errDataNotEnough {
				c.handshakeStatusAsync = stateServerHandshakeReadFinishedReadChangeCipherSpec
			}
			return err
		}
		c.handshakeStatusAsync = stateServerHandshakeReadFinishedReadChangeCipherSpec
	}

	if c.handshakeStatusAsync >= stateServerHandshakeReadFinishedDone {
		return nil
	}
	msg, err := c.readHandshake()
	if err != nil {
		if err != errDataNotEnough {
			c.handshakeStatusAsync = stateServerHandshakeReadFinishedDone
		}
		return err
	}
	clientFinished, ok := msg.(*finishedMsg)
	if !ok {
		c.sendAlert(alertUnexpectedMessage)
		c.handshakeStatusAsync = stateServerHandshakeReadFinishedDone
		return unexpectedMessageError(clientFinished, msg)
	}

	verify := hs.finishedHash.clientSum(hs.masterSecret)
	if len(verify) != len(clientFinished.verifyData) ||
		subtle.ConstantTimeCompare(verify, clientFinished.verifyData) != 1 {
		c.sendAlert(alertHandshakeFailure)
		c.handshakeStatusAsync = stateServerHandshakeReadFinishedDone
		return errors.New("tls: client's Finished message is incorrect")
	}

	hs.finishedHash.Write(clientFinished.marshal())
	copy(out, verify)

	c.handshakeStatusAsync = stateServerHandshakeReadFinishedDone
	return nil
}

func (hs *serverHandshakeState) sendSessionTicket() error {
	// ticketSupported is set in a resumption handshake if the
	// ticket from the client was encrypted with an old session
	// ticket key and thus a refreshed ticket should be sent.
	if !hs.hello.ticketSupported {
		return nil
	}

	c := hs.c

	if c.handshakeStatusAsync >= stateServerHandshakeSendSessionTicket {
		return nil
	}
	c.handshakeStatusAsync = stateServerHandshakeSendSessionTicket

	m := new(newSessionTicketMsg)

	createdAt := uint64(c.config.time().Unix())
	if hs.sessionState != nil {
		// If this is re-wrapping an old key, then keep
		// the original time it was created.
		createdAt = hs.sessionState.createdAt
	}

	var certsFromClient [][]byte
	for _, cert := range c.peerCertificates {
		certsFromClient = append(certsFromClient, cert.Raw)
	}
	state := sessionState{
		vers:         c.vers,
		cipherSuite:  hs.suite.id,
		createdAt:    createdAt,
		masterSecret: hs.masterSecret,
		certificates: certsFromClient,
	}
	var err error
	m.ticket, err = c.encryptTicket(state.marshal())
	if err != nil {
		return err
	}

	hs.finishedHash.Write(m.marshal())
	if _, err := c.writeRecord(recordTypeHandshake, m.marshal()); err != nil {
		return err
	}

	return nil
}

func (hs *serverHandshakeState) sendSessionTicket2() error {
	// ticketSupported is set in a resumption handshake if the
	// ticket from the client was encrypted with an old session
	// ticket key and thus a refreshed ticket should be sent.
	if !hs.hello.ticketSupported {
		return nil
	}

	c := hs.c

	if c.handshakeStatusAsync >= stateServerHandshakeSendSessionTicket2 {
		return nil
	}
	c.handshakeStatusAsync = stateServerHandshakeSendSessionTicket2

	m := new(newSessionTicketMsg)

	createdAt := uint64(c.config.time().Unix())
	if hs.sessionState != nil {
		// If this is re-wrapping an old key, then keep
		// the original time it was created.
		createdAt = hs.sessionState.createdAt
	}

	var certsFromClient [][]byte
	for _, cert := range c.peerCertificates {
		certsFromClient = append(certsFromClient, cert.Raw)
	}
	state := sessionState{
		vers:         c.vers,
		cipherSuite:  hs.suite.id,
		createdAt:    createdAt,
		masterSecret: hs.masterSecret,
		certificates: certsFromClient,
	}
	var err error
	m.ticket, err = c.encryptTicket(state.marshal())
	if err != nil {
		return err
	}

	hs.finishedHash.Write(m.marshal())
	if _, err := c.writeRecord(recordTypeHandshake, m.marshal()); err != nil {
		return err
	}

	return nil
}

func (hs *serverHandshakeState) sendFinished(out []byte) error {
	c := hs.c

	if c.handshakeStatusAsync >= stateServerHandshakeSendFinished {
		return nil
	}
	c.handshakeStatusAsync = stateServerHandshakeSendFinished

	if _, err := c.writeRecord(recordTypeChangeCipherSpec, []byte{1}); err != nil {
		return err
	}

	finished := new(finishedMsg)
	finished.verifyData = hs.finishedHash.serverSum(hs.masterSecret)
	hs.finishedHash.Write(finished.marshal())
	if _, err := c.writeRecord(recordTypeHandshake, finished.marshal()); err != nil {
		return err
	}

	copy(out, finished.verifyData)

	return nil
}

func (hs *serverHandshakeState) sendFinished2(out []byte) error {
	c := hs.c

	if c.handshakeStatusAsync >= stateServerHandshakeSendFinished2 {
		return nil
	}
	c.handshakeStatusAsync = stateServerHandshakeSendFinished2

	if _, err := c.writeRecord(recordTypeChangeCipherSpec, []byte{1}); err != nil {
		return err
	}

	finished := new(finishedMsg)
	finished.verifyData = hs.finishedHash.serverSum(hs.masterSecret)
	hs.finishedHash.Write(finished.marshal())
	if _, err := c.writeRecord(recordTypeHandshake, finished.marshal()); err != nil {
		return err
	}

	copy(out, finished.verifyData)

	return nil
}

// processCertsFromClient takes a chain of client certificates either from a
// Certificates message or from a sessionState and verifies them. It returns
// the public key of the leaf certificate.
func (c *Conn) processCertsFromClient(certificate Certificate) error {
	certificates := certificate.Certificate
	certs := make([]*x509.Certificate, len(certificates))
	var err error
	for i, asn1Data := range certificates {
		if certs[i], err = x509.ParseCertificate(asn1Data); err != nil {
			c.sendAlert(alertBadCertificate)
			return errors.New("tls: failed to parse client certificate: " + err.Error())
		}
	}

	if len(certs) == 0 && requiresClientCert(c.config.ClientAuth) {
		c.sendAlert(alertBadCertificate)
		return errors.New("tls: client didn't provide a certificate")
	}

	if c.config.ClientAuth >= VerifyClientCertIfGiven && len(certs) > 0 {
		opts := x509.VerifyOptions{
			Roots:         c.config.ClientCAs,
			CurrentTime:   c.config.time(),
			Intermediates: x509.NewCertPool(),
			KeyUsages:     []x509.ExtKeyUsage{x509.ExtKeyUsageClientAuth},
		}

		for _, cert := range certs[1:] {
			opts.Intermediates.AddCert(cert)
		}

		chains, err := certs[0].Verify(opts)
		if err != nil {
			c.sendAlert(alertBadCertificate)
			return errors.New("tls: failed to verify client certificate: " + err.Error())
		}

		c.verifiedChains = chains
	}

	c.peerCertificates = certs
	c.ocspResponse = certificate.OCSPStaple
	c.scts = certificate.SignedCertificateTimestamps

	if len(certs) > 0 {
		switch certs[0].PublicKey.(type) {
		case *ecdsa.PublicKey, *rsa.PublicKey, ed25519.PublicKey:
		default:
			c.sendAlert(alertUnsupportedCertificate)
			return fmt.Errorf("tls: client certificate contains an unsupported public key of type %T", certs[0].PublicKey)
		}
	}

	if c.config.VerifyPeerCertificate != nil {
		if err := c.config.VerifyPeerCertificate(certificates, c.verifiedChains); err != nil {
			c.sendAlert(alertBadCertificate)
			return err
		}
	}

	return nil
}

func clientHelloInfo(c *Conn, clientHello *clientHelloMsg) *ClientHelloInfo {
	supportedVersions := clientHello.supportedVersions
	if len(clientHello.supportedVersions) == 0 {
		supportedVersions = supportedVersionsFromMax(clientHello.vers)
	}

	return &ClientHelloInfo{
		CipherSuites:      clientHello.cipherSuites,
		ServerName:        clientHello.serverName,
		SupportedCurves:   clientHello.supportedCurves,
		SupportedPoints:   clientHello.supportedPoints,
		SignatureSchemes:  clientHello.supportedSignatureAlgorithms,
		SupportedProtos:   clientHello.alpnProtocols,
		SupportedVersions: supportedVersions,
		Conn:              c.conn,
		config:            c.config,
	}
}
