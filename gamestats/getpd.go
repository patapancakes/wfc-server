package gamestats

import (
	"database/sql"
	"strconv"
	"strings"
	"time"
	"wwfc/common"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

func (g *GameStatsSession) getpd(command common.GameSpyCommand) {
	errMsg := common.GameSpyCommand{
		Command:      "getpdr",
		CommandValue: "0",
		OtherValues: map[string]string{
			"pid": command.OtherValues["pid"],
			"lid": strconv.Itoa(g.LoginID),
		},
	}

	profileId, err := strconv.ParseUint(command.OtherValues["pid"], 10, 32)
	if err != nil {
		logging.Error(g.ModuleName, "Invalid pid:", aurora.Cyan(profileId))
		logging.Error(g.ModuleName, "Full command:", command)
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

	logging.Info(g.ModuleName, "Get persist data: PID:", aurora.Cyan(profileId), "Type:", aurora.Cyan(ptype), "Index:", aurora.Cyan(dindex))

	if (ptype == PrivateRead || ptype == PrivateReadWrite) && (profileId != uint64(g.Profile.ID)) {
		logging.Warn(g.ModuleName, "Private persist data access attempt for other profile")
		g.Write(errMsg)
		return
	}

	var data string
	var modified time.Time
	if command.OtherValues["kv"] == "1" {
		keys, ok := command.OtherValues["keys"]
		if !ok {
			logging.Error(g.ModuleName, "Missing keys")
			logging.Error(g.ModuleName, "Full command:", command)
			g.Write(errMsg)
			return
		}

		var kv common.KeyValues
		kv, modified, err = db.GetGameStatsPersistDataKV(uint32(profileId), ptype, dindex, strings.Split(keys, string(byte(1))))

		data = kv.Encode()
	} else {
		data, modified, err = db.GetGameStatsPersistData(uint32(profileId), ptype, dindex)
	}
	if err != nil {
		if err != sql.ErrNoRows {
			logging.Error(g.ModuleName, "GetGameStatsPersistData returned", err)
			g.Write(errMsg)
			return
		}

		logging.Warn(g.ModuleName, "No data found")
		g.Write(errMsg)
		return
	}

	modStr, ok := command.OtherValues["mod"]
	if ok {
		mod, err := strconv.Atoi(modStr)
		if err != nil {
			logging.Error(g.ModuleName, "Invalid mod:", aurora.Cyan(profileId))
			logging.Error(g.ModuleName, "Full command:", command)
			g.Write(errMsg)
			return
		}

		// if modified before mod, return empty success
		if modified.Before(time.Unix(int64(mod), 0)) {
			data = ""
		}
	}

	g.Write(common.GameSpyCommand{
		Command:      "getpdr",
		CommandValue: "1",
		OtherValues: map[string]string{
			"lid":    strconv.Itoa(g.LoginID),
			"pid":    command.OtherValues["pid"],
			"mod":    strconv.Itoa(int(modified.Unix())),
			"length": strconv.Itoa(len(data)),
			"data":   data,
		},
	})
}
