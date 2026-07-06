package gpcm

import (
	"strconv"
	"wwfc/common"
	"wwfc/database"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

func (g *GameSpySession) getProfile(command common.GameSpyCommand) {
	strProfileId := command.OtherValues["profileid"]
	profileId, err := strconv.ParseUint(strProfileId, 10, 32)
	if err != nil {
		// There was an error getting profile info.
		g.replyError(ErrGetProfile)
		return
	}

	logging.Info(g.ModuleName, "Looking up the profile of", aurora.Cyan(profileId).String())

	profile := database.Profile{}
	locstring := ""

	mutex.Lock()
	if session, ok := sessions[uint32(profileId)]; ok && session.LoggedIn {
		locstring = session.LocString
		profile = session.Profile
		mutex.Unlock()
	} else {
		mutex.Unlock()
		profile, ok = db.GetProfile(uint32(profileId))
		if !ok {
			// The profile info was requested on is invalid.
			g.replyError(ErrGetProfileBadProfile)
			return
		}
	}

	if profile.ProfileId == g.Profile.ProfileId {
		g.WriteBuffer += common.CreateGameSpyMessage(common.GameSpyCommand{
			Command:      "pi",
			CommandValue: "",
			OtherValues: map[string]string{
				"profileid":  command.OtherValues["profileid"],
				"nick":       profile.UniqueNick,
				"userid":     strconv.FormatUint(uint64(profile.UserId), 10),
				"email":      profile.Email,
				"sig":        common.RandomHexString(32),
				"uniquenick": profile.UniqueNick,
				"firstname":  profile.FirstName,
				"lastname":   profile.LastName,
				"pid":        "11",
				"lon":        "0.000000",
				"lat":        "0.000000",
				"loc":        locstring,
				"id":         command.OtherValues["id"],
			},
		})
	} else {
		g.WriteBuffer += common.CreateGameSpyMessage(common.GameSpyCommand{
			Command:      "pi",
			CommandValue: "",
			OtherValues: map[string]string{
				"profileid":  command.OtherValues["profileid"],
				"nick":       "000000000" + profile.GsbrCode[:4] + "0000000",
				"userid":     "0",
				"email":      "000000000" + profile.GsbrCode[:4] + "0000000" + "@nds",
				"sig":        common.RandomHexString(32),
				"uniquenick": "000000000" + profile.GsbrCode[:4] + "0000000",
				"firstname":  profile.FirstName,
				"lastname":   "000000000" + profile.GsbrCode[:4] + "0000000",
				"pid":        "11",
				"lon":        "0.000000",
				"lat":        "0.000000",
				"loc":        locstring,
				"id":         command.OtherValues["id"],
			},
		})
	}
}

func (g *GameSpySession) updateProfile(command common.GameSpyCommand) {
	if openHost, ok := command.OtherValues["wl:oh"]; ok {
		enabled := openHost != "0"
		if !g.Profile.OpenHost && enabled {
			g.openHostEnabled(true, true)
		} else if g.Profile.OpenHost && !enabled {
			g.openHostDisabled()
		}
	}

	db.UpdateProfile(&g.Profile, command.OtherValues)
}

func VerifyPlayerSearch(profileId uint32, sessionKey int32, gameName string) (string, bool) {
	mutex.Lock()
	defer mutex.Unlock()

	if session, ok := sessions[profileId]; ok && session.LoggedIn && session.SessionKey == sessionKey && session.GameName == gameName {
		return "000000000" + session.Profile.GsbrCode[:4] + "0000000", true
	}

	return "", false
}
