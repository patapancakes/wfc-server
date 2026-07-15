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

func (c *Connection) LoginUserToGPCM(userId uint64, gsbrcd string, ipAddress string, ingamesn string) (Profile, error) {
	profile := Profile{
		UserID:   userId,
		GsbrCode: gsbrcd,
	}

	var exists bool
	err := c.pool.QueryRowContext(c.ctx, DoesProfileExist, userId, gsbrcd).Scan(&exists)
	if err != nil {
		return Profile{}, err
	}
	if !exists {
		// Create the GPCM account
		profile.ID, err = c.CreateProfile(profile)
		if err != nil {
			logging.Error("DATABASE", "Error creating profile:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(profile.ID), "\nerror:", err.Error())
			return Profile{}, err
		}

		logging.Notice("DATABASE", "Created new GPCM profile:", aurora.Cyan(userId), aurora.Cyan(gsbrcd), aurora.Cyan(profile.ID))
		profile.Created = true
	} else {
		var firstName sql.NullString
		var lastName sql.NullString

		err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&profile.ID, &firstName, &lastName)
		if err != nil {
			return Profile{}, err
		}

		profile.FirstName = firstName.String
		profile.LastName = lastName.String

		logging.Notice("DATABASE", "Log in GPCM profile:", aurora.Cyan(userId), aurora.Cyan(profile.GsbrCode), "-", aurora.Cyan(profile.ID))
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
	err = c.pool.QueryRowContext(c.ctx, SearchProfileBan, profile.ID, ipAddress, timeNow).Scan(&banExists, &banTOS, &banReason)
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

	err := c.pool.QueryRowContext(c.ctx, GetUserProfileID, userId, gsbrcd).Scan(&profile.ID, &firstName, &lastName)
	if err != nil {
		return Profile{}, err
	}

	profile.FirstName = firstName.String
	profile.LastName = lastName.String

	return profile, nil
}
