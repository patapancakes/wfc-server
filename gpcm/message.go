package gpcm

import (
	"encoding/binary"
	"encoding/hex"
	"fmt"
	"owfc/common"
	"owfc/logging"
	"owfc/qr2"
	"strconv"
	"strings"

	"github.com/logrusorgru/aurora/v3"
)

func (g *GameSpySession) buddyMessage(command common.GameSpyCommand) {
	// TODO: There are other command values that mean the same thing
	if command.CommandValue != strconv.Itoa(BuddyMessage) {
		logging.Error(g.ModuleName, "Received unknown buddy message type:", aurora.Cyan(command.CommandValue))
		return
	}

	strToProfileId := command.OtherValues["t"]
	toProfileId, err := strconv.ParseUint(strToProfileId, 10, 32)
	if err != nil {
		logging.Error(g.ModuleName, "Invalid profile ID string:", aurora.Cyan(strToProfileId))
		g.replyError(ErrMessage)
		return
	}

	if !g.isFriendAuthorized(uint32(toProfileId)) {
		logging.Error(g.ModuleName, "Destination", aurora.Cyan(toProfileId), "is not even on sender's friend list")
		g.replyError(ErrMessageNotFriends)
		return
	}

	msg, ok := command.OtherValues["msg"]
	if !ok || msg == "" {
		logging.Error(g.ModuleName, "Missing message value")
		g.replyError(ErrMessage)
		return
	}

	// Parse message for security and room tracking purposes
	var version int
	var msgDataIndex int

	switch {
	case strings.HasPrefix(msg, "GPCM3vMAT"):
		version = 3
		msgDataIndex = 9
	case strings.HasPrefix(msg, "GPCM11vMAT"):
		// Only used for Brawl
		version = 11
		msgDataIndex = 10
	case strings.HasPrefix(msg, "GPCM90vMAT"):
		version = 90
		msgDataIndex = 10
	default:
		logging.Error(g.ModuleName, "Invalid message prefix; message:", msg)
		g.replyError(ErrMessage)
		return
	}

	if len(msg) < msgDataIndex+1 {
		logging.Error(g.ModuleName, "Invalid message length; message:", msg)
		g.replyError(ErrMessage)
		return
	}

	cmd := msg[msgDataIndex]
	msgDataIndex++

	var msgData []byte

	switch version {
	case 3:
		if len(msg) == msgDataIndex {
			break
		}

		for stringValue := range strings.SplitSeq(msg[msgDataIndex:], "/") {
			intValue, err := strconv.ParseUint(stringValue, 10, 32)
			if err != nil {
				logging.Error(g.ModuleName, "Invalid message value; message:", msg)
				g.replyError(ErrMessage)
				return
			}

			msgData = binary.LittleEndian.AppendUint32(msgData, uint32(intValue))
		}

	case 11:
		if len(msg) == msgDataIndex {
			break
		}

		for stringValue := range strings.SplitSeq(msg[msgDataIndex:], "/") {
			byteValue, err := hex.DecodeString(stringValue)
			if err != nil || len(byteValue) != 4 {
				logging.Error(g.ModuleName, "Invalid message value; message:", msg)
				g.replyError(ErrMessage)
				return
			}

			msgData = append(msgData, byteValue...)
		}

	case 90:
		if len(msg) == msgDataIndex {
			break
		}

		msgData, err = common.Base64DwcEncoding.DecodeString(msg[msgDataIndex:])
		if err != nil {
			logging.Error(g.ModuleName, "Invalid message base64 data; message:", msg)
			g.replyError(ErrMessage)
			return
		}

	default:
		logging.Error(g.ModuleName, "Invalid message version; message:", msg)
		g.replyError(ErrMessage)
		return
	}

	if len(msgData) > 0x200 || (len(msgData)&3) != 0 {
		logging.Error(g.ModuleName, "Invalid length message data; message:", msg)
		g.replyError(ErrMessage)
		return
	}

	msgMatchData, ok := common.DecodeMatchCommand(cmd, msgData, version)
	common.LogMatchCommand(g.ModuleName, strconv.FormatInt(int64(toProfileId), 10), cmd, msgMatchData)
	if !ok {
		logging.Error(g.ModuleName, "Invalid match command data; message:", msg)
		g.replyError(ErrMessage)
		return
	}

	switch cmd {
	case common.MatchReservation:
		g.QR2IP = uint64(msgMatchData.Reservation.PublicIP) | (uint64(msgMatchData.Reservation.PublicPort) << 32)
	case common.MatchResvOK:
		g.QR2IP = uint64(msgMatchData.ResvOK.PublicIP) | (uint64(msgMatchData.ResvOK.PublicPort) << 32)
	}

	mutex.Lock()
	defer mutex.Unlock()

	var toSession *GameSpySession
	if toSession, ok = sessions[uint32(toProfileId)]; !ok || !toSession.LoggedIn {
		logging.Error(g.ModuleName, "Destination", aurora.Cyan(toProfileId), "is not online")
		g.replyError(ErrMessageFriendOffline)
		return
	}

	if toSession.GameName != g.GameName {
		logging.Error(g.ModuleName, "Destination", aurora.Cyan(toProfileId), "is not playing the same game")
		g.replyError(ErrMessage)
		return
	}

	sameAddress := strings.Split(g.RemoteAddr, ":")[0] == strings.Split(toSession.RemoteAddr, ":")[0]

	ok = true
	switch cmd {
	case common.MatchReservation:
		ok = g.mungeMatchReservation(toSession, &msgMatchData, uint32(toProfileId), sameAddress)

	case common.MatchResvOK, common.MatchResvDeny, common.MatchResvWait:
		ok = g.mungeMatchReservationResult(cmd, toSession, &msgMatchData, uint32(toProfileId), sameAddress)

	case common.MatchTellAddr:
		if g.QR2IP == 0 || toSession.QR2IP == 0 {
			logging.Error(g.ModuleName, "Missing QR2 IP")
			g.replyError(ErrMessage)
			ok = false
			break
		}

		qr2.ProcessGPTellAddr(g.Profile.ID, g.QR2IP, toSession.Profile.ID, toSession.QR2IP)
	}

	if !ok {
		return
	}

	newMsg, ok := common.EncodeMatchCommand(cmd, msgMatchData)
	if !ok || len(newMsg) > 0x200 || (len(newMsg)%4) != 0 {
		logging.Error(g.ModuleName, "Failed to encode match command; message:", msg)
		g.replyError(ErrMessage)
		return
	}

	if cmd == common.MatchReservation {
		g.Reservation = msgMatchData
		g.ReservationPID = uint32(toProfileId)
	}

	var newMsgStr string

	// Re-encode the new message
	switch version {
	case 3:
		newMsgStr = "GPCM3vMAT" + string(cmd)

		for i := 0; i < len(newMsg); i += 4 {
			if i > 0 {
				newMsgStr += "/"
			}

			newMsgStr += strconv.FormatUint(uint64(binary.LittleEndian.Uint32(newMsg[i:])), 10)
		}

	case 11:
		newMsgStr = "GPCM11vMAT" + string(cmd)

		for i := 0; i < len(newMsg); i += 4 {
			if i > 0 {
				newMsgStr += "/"
			}

			newMsgStr += hex.EncodeToString(newMsg[i : i+4])
		}

	case 90:
		newMsgStr = "GPCM90vMAT" + string(cmd) + common.Base64DwcEncoding.EncodeToString(newMsg)
	}

	sendMessageToSession(BuddyMessage, g.Profile.ID, toSession, newMsgStr)
}

func (g *GameSpySession) mungeMatchReservation(toSession *GameSpySession, msgMatchData *common.MatchCommandData, toProfileId uint32, sameAddress bool) bool {
	if g.QR2IP == 0 {
		logging.Error(g.ModuleName, "Missing QR2 IP")
		g.replyError(ErrMessage)
		return false
	}

	if !sameAddress {
		searchId := qr2.GetSearchID(g.QR2IP)
		msgMatchData.Reservation.PublicIP = uint32(searchId & 0xffffffff)
		msgMatchData.Reservation.PublicPort = uint16(searchId >> 32)
		msgMatchData.Reservation.LocalIP = 0
		msgMatchData.Reservation.LocalPort = 0
	}

	return true
}

func (g *GameSpySession) mungeMatchReservationResult(cmd byte, toSession *GameSpySession, msgMatchData *common.MatchCommandData, toProfileId uint32, sameAddress bool) bool {
	if toSession.ReservationPID != g.Profile.ID || toSession.Reservation.Reservation == nil {
		logging.Error(g.ModuleName, "Destination", aurora.Cyan(toProfileId), "has no reservation with the sender")
		// Allow the message through anyway to avoid a room deadlock
	}

	if toSession.Reservation.Version != msgMatchData.Version {
		logging.Error(g.ModuleName, "Reservation version mismatch")
		g.replyError(ErrMessage)
		return false
	}

	if cmd != common.MatchResvOK {
		if toSession.ReservationPID == g.Profile.ID {
			toSession.ReservationPID = 0
		}
		return true
	}

	if g.QR2IP == 0 || toSession.QR2IP == 0 {
		logging.Error(g.ModuleName, "Missing QR2 IP")
		g.replyError(ErrMessage)
		return false
	}

	if !qr2.ProcessGPResvOK(msgMatchData.Version, *toSession.Reservation.Reservation, *msgMatchData.ResvOK, g.QR2IP, g.Profile.ID, toSession.QR2IP, uint32(toProfileId)) {
		g.replyError(ErrMessage)
		return false
	}

	if !sameAddress {
		searchId := qr2.GetSearchID(g.QR2IP)
		if searchId == 0 {
			logging.Error(g.ModuleName, "Could not get QR2 search ID for IP", aurora.Cyan(fmt.Sprintf("%016x", g.QR2IP)))
			g.replyError(ErrMessage)
			return false
		}

		msgMatchData.ResvOK.PublicIP = uint32(searchId & 0xffffffff)
		msgMatchData.ResvOK.PublicPort = uint16(searchId >> 32)
	}
	return true
}
