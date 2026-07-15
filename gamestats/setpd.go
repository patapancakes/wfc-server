package gamestats

import (
	"owfc/common"
	"owfc/logging"
	"strconv"
	"strings"
	"time"

	"github.com/logrusorgru/aurora/v3"
)

func (g *GameStatsSession) setpd(command common.GameSpyCommand) {
	// Example (with formatting):
	// \setpd\
	//     \pid\1000000004
	//     \ptype\3
	//     \dindex\0
	//     \kv\1
	//     \lid\0
	//     \length\149
	//     \data\
	//         \itast_friend_p\AFAAYQBsAGEAcABlAGwAaQAAAAAAANzmAFAAYQBsAGEAcABlAGwAaQAAAABSJYgbuuQEbA5pAASOoAk9JpJsjKhAFEmQTQCKAIolBAAAAAAAAAAAAAAAAAAAAAAAAAAAGps*\x00
	// \final\

	errMsg := common.GameSpyCommand{
		Command:      "setpdr",
		CommandValue: "0",
		OtherValues: map[string]string{
			"pid": command.OtherValues["pid"],
			"lid": strconv.Itoa(g.LoginID),
		},
	}

	if command.OtherValues["pid"] != strconv.FormatUint(uint64(g.Profile.ID), 10) {
		logging.Error(g.ModuleName, "Invalid profile ID:", aurora.Cyan(command.OtherValues["pid"]))
		g.Write(errMsg)
		return
	}

	ptype, err := strconv.Atoi(command.OtherValues["ptype"])
	if err != nil || ptype > 3 || (ptype != PublicReadWrite && ptype != PrivateReadWrite) {
		logging.Error(g.ModuleName, "Invalid ptype:", aurora.Cyan(ptype))
		logging.Error(g.ModuleName, "Full command:", command)
		g.Write(errMsg)
		return
	}

	dindex, err := strconv.Atoi(command.OtherValues["dindex"])
	if err != nil {
		logging.Error(g.ModuleName, "Invalid dindex:", aurora.Cyan(dindex))
		logging.Error(g.ModuleName, "Full command:", command)
		g.Write(errMsg)
		return
	}

	dataStr, ok := command.OtherValues["data"]
	if !ok {
		logging.Error(g.ModuleName, "Missing data")
		logging.Error(g.ModuleName, "Full command:", command)
		g.Write(errMsg)
		return
	}

	logging.Info(g.ModuleName, "Set persist data: PID:", aurora.Cyan(g.Profile.ID), "Type:", aurora.Cyan(ptype), "Index:", aurora.Cyan(dindex), "Data:", aurora.Cyan(dataStr))

	// Trim extra null byte at the end
	dataStr = strings.TrimSuffix(dataStr, "\x00")

	if strings.ContainsRune(dataStr, 0) {
		logging.Error(g.ModuleName, "Data contains null byte")
		g.Write(errMsg)
		return
	}

	var modifiedTime time.Time
	if command.OtherValues["kv"] == "1" {
		modifiedTime, err = db.SetGameStatsPersistDataKV(g.Profile.ID, ptype, dindex, common.KeyValuesFromString(dataStr))
	} else {
		modifiedTime, err = db.SetGameStatsPersistData(g.Profile.ID, ptype, dindex, dataStr)
	}
	if err != nil {
		logging.Error(g.ModuleName, "SetGameStatsPersistData returned", err)
		g.Write(errMsg)
		return
	}

	// TODO: Is mod supposed to be the last modified time or new modified time?
	g.Write(common.GameSpyCommand{
		Command:      "setpdr",
		CommandValue: "1",
		OtherValues: map[string]string{
			"lid": strconv.Itoa(g.LoginID),
			"pid": command.OtherValues["pid"],
			"mod": strconv.Itoa(int(modifiedTime.Unix())),
		},
	})
}
