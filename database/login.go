package database

import (
	"database/sql"
	"errors"
	"time"
	"wwfc/logging"

	"github.com/logrusorgru/aurora/v3"
)

var (
	ErrProfileBannedTOS = errors.New("profile is banned for violating the Terms of Service")
)

func (c *Connection) LoginUserToGPCM(userId uint64, gsbrcd string, profileId uint32, ipAddress string, ingamesn string) (Profile, error) {
	var exists bool
	err := c.pool.QueryRowContext(c.ctx, DoesProfileExist, userId, gsbrcd).Scan(&exists)
	if err != nil {
		return Profile{}, err
	}

	profile := Profile{
		UserID:   userId,
		GsbrCode: gsbrcd,
	}

	var lastIpAddress string

	if !exists {
		profile.ID = profileId

		// Create the GPCM account
		err := c.CreateProfile(&profile)
		if err != nil {
			logging.Error("DATABASE", "Error creating profile:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(profile.ID), "\nerror:", err.Error())
			return Profile{}, err
		}

		logging.Notice("DATABASE", "Created new GPCM profile:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(profile.ID))
		profile.Created = true
	} else {
		var firstName sql.NullString
		var lastName sql.NullString
		var ip sql.NullString

		err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&profile.ID, &firstName, &lastName, &ip)
		if err != nil {
			return Profile{}, err
		}

		profile.FirstName = firstName.String
		profile.LastName = lastName.String
		lastIpAddress = ip.String

		if profileId != 0 && profile.ID != profileId {
			err := c.UpdateProfileID(&profile, profileId)
			if err != nil {
				logging.Warn("DATABASE", "Could not update", aurora.Cyan(userId), aurora.Cyan(gsbrcd), "profile ID from", aurora.Cyan(profile.ID), "to", aurora.Cyan(profileId))
			} else {
				logging.Notice("DATABASE", "Updated GPCM profile ID:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(profile.ID))
			}
		}

		logging.Notice("DATABASE", "Log in GPCM profile:", aurora.Cyan(userId), aurora.Cyan(profile.GsbrCode), "-", aurora.Cyan(profile.ID))
	}

	// This should be set if the user already knows its own profile ID
	if profileId != 0 && profile.LastName == "" {
		c.UpdateProfile(&profile, map[string]string{
			"lastname": "000000000" + gsbrcd,
		})
	}

	// Update the user's last IP address and ingamesn
	_, err = c.pool.ExecContext(c.ctx, UpdateProfileLastIPAddress, ipAddress, ingamesn, profile.ID)
	if err != nil {
		return Profile{}, err
	}

	// Find ban from device ID or IP address
	var banExists bool
	var banTOS bool
	var banReason string
	timeNow := time.Now().UTC()
	err = c.pool.QueryRowContext(c.ctx, SearchProfileBan, profile.ID, ipAddress, lastIpAddress, timeNow).Scan(&banExists, &banTOS, &banReason)

	if err != nil {
		if err != sql.ErrNoRows {
			return Profile{}, err
		}

		banExists = false
	}

	if banExists {
		if banTOS {
			logging.Warn("DATABASE", "Profile", aurora.Cyan(profile.ID), "is banned")
			return Profile{BanReason: banReason}, ErrProfileBannedTOS
		}

		logging.Warn("DATABASE", "Profile", aurora.Cyan(profile.ID), "is restricted")
		profile.Restricted = true
		profile.BanReason = banReason
	}

	return profile, nil
}

func (c *Connection) LoginUserToGameStats(userId uint64, gsbrcd string) (Profile, error) {
	profile := Profile{
		UserID:   userId,
		GsbrCode: gsbrcd,
	}

	var firstName sql.NullString
	var lastName sql.NullString
	var lastIPAddress sql.NullString

	err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&profile.ID, &firstName, &lastName, &lastIPAddress)
	if err != nil {
		return Profile{}, err
	}

	profile.FirstName = firstName.String
	profile.LastName = lastName.String
	profile.LastIPAddress = lastIPAddress.String

	return profile, nil
}
