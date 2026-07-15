package qr2

import (
	"encoding/base64"
	"fmt"
	"net"
	"owfc/common"
	"owfc/logging"
	"strconv"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

func sendChallenge(conn net.PacketConn, addr net.UDPAddr, session Session, lookupAddr uint64) {
	challenge := session.Challenge
	if challenge == "" {
		// Generate challenge
		addrString := strings.Split(addr.String(), ":")
		var hexIP strings.Builder
		for i := range strings.SplitSeq(addrString[0], ".") {
			val, err := strconv.ParseUint(i, 10, 64)
			if err != nil {
				panic(err)
			}

			hexIP.WriteString(fmt.Sprintf("%02X", val))
		}

		port, err := strconv.ParseUint(addrString[1], 10, 64)
		if err != nil {
			panic(err)
		}

		hexPort := fmt.Sprintf("%04X", port)

		challenge = common.RandomString(6) + "00" + hexIP.String() + hexPort
		mutex.Lock()
		if sessionPtr := sessions[lookupAddr]; sessionPtr != nil {
			sessionPtr.Challenge = challenge
		} else {
			mutex.Unlock()
			return
		}
		mutex.Unlock()
	}

	response := createResponseHeader(ChallengeRequest, session.SessionID)
	response = append(response, []byte(challenge)...)
	response = append(response, 0)

	go func() {
		for range 5 {
			_, err := conn.WriteTo(response, &addr)

			time.Sleep(1 * time.Second)

			if err != nil {
				continue
			}

			mutex.Lock()
			session, ok := sessions[lookupAddr]
			if !ok || session.Authenticated || session.LastKeepAlive < time.Now().UTC().Unix()-60 {
				mutex.Unlock()
				return
			}
			addr = session.Addr
			mutex.Unlock()
		}
		logging.Info("QR2", "Failed to send challenge to", aurora.Cyan(addr.String()))
		mutex.Lock()
		defer mutex.Unlock()
		removeSession(lookupAddr)
	}()
}

func encode(data []byte) string {
	padded := make([]byte, ((len(data)+2)/3)*3)
	copy(padded, data)

	return base64.StdEncoding.EncodeToString(padded)
}

func encrypt(key []byte, data []byte) []byte {
	var state [256]byte

	swap := func(a *byte, b *byte) {
		swap := *a
		*a = *b
		*b = swap
	}

	for counter := range state {
		state[counter] = byte(counter)
	}

	var x, y uint8
	for counter := range state {
		y = key[x] + state[counter] + y
		x = (x + 1) % uint8(len(key))
		swap(&state[counter], &state[y])
	}

	x = 0
	y = 0

	var xorIdx uint8
	for counter := range data {
		x = x + data[counter] + 1
		y = state[x] + y
		swap(&state[x], &state[y])
		xorIdx = state[x] + state[y]
		data[counter] ^= state[xorIdx]
	}

	return data
}
