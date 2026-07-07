package database

import (
	"errors"
	"time"
	"wwfc/common"
)

const (
	InsertProfile              = `INSERT INTO profiles (user_id, gsbrcd) VALUES (?, ?) RETURNING id`
	InsertProfileWithID        = `INSERT INTO profiles (id, user_id, gsbrcd) VALUES (?, ?, ?)`
	UpdateProfileTable         = `UPDATE profiles SET firstname = CASE WHEN ? THEN ? ELSE firstname END, lastname = CASE WHEN ? THEN ? ELSE lastname END WHERE id = ?`
	UpdateUserProfileID        = `UPDATE profiles SET id = ? WHERE user_id = ? AND gsbrcd = ?`
	GetProfile                 = `SELECT user_id, gsbrcd, firstname, lastname, last_ip_address, last_ingamesn FROM profiles WHERE id = ?`
	ClearProfileQuery          = `DELETE FROM profiles WHERE id = ? RETURNING user_id, gsbrcd, firstname, lastname, last_ip_address, last_ingamesn`
	DoesProfileExist           = `SELECT EXISTS(SELECT 1 FROM profiles WHERE user_id = ? AND gsbrcd = ?)`
	IsProfileIDInUse           = `SELECT EXISTS(SELECT 1 FROM profiles WHERE id = ?)`
	DeleteProfileSession       = `DELETE FROM sessions WHERE id = ?`
	GetUserProfileID           = `SELECT id, firstname, lastname, last_ip_address FROM profiles WHERE user_id = ? AND gsbrcd = ?`
	UpdateProfileLastIPAddress = `UPDATE profiles SET last_ip_address = ?, last_ingamesn = ? WHERE id = ?`
	UpdateProfileBan           = `UPDATE profiles SET has_ban = true, ban_issued = ?, ban_expires = ?, ban_reason = ?, ban_reason_hidden = ?, ban_moderator = ?, ban_tos = ? WHERE id = ?`
	SearchProfileBan           = `SELECT has_ban, ban_tos FROM profiles WHERE has_ban = true AND (id = ? OR last_ip_address = ?) AND (ban_expires IS NULL OR ban_expires > ?) AND (ban_expires IS NULL OR ban_expires > ?) ORDER BY ban_tos DESC LIMIT 1`
	SearchProfileBanInfo       = `SELECT has_ban, ban_tos, ban_issued, ban_expires, ban_reason, id, gsbrcd, last_ingamesn FROM profiles WHERE has_ban = true AND (id = ? OR last_ip_address = ?) ORDER BY ban_expires DESC LIMIT 1`
	DisableProfileBan          = `UPDATE profiles SET has_ban = false WHERE id = ?`
)

type Profile struct {
	ID            uint32
	UserID        uint64
	GsbrCode      string
	FirstName     string
	LastName      string
	Restricted    bool
	BanReason     string
	LastInGameSn  string
	LastIPAddress string
	Created       bool
}

func (p Profile) Email() string {
	return p.UniqueNick() + "@nds"
}

func (p Profile) UniqueNick() string {
	return common.Base32Encode(p.UserID) + p.GsbrCode
}

var (
	ErrProfileIDInUse         = errors.New("profile ID is already in use")
	ErrReservedProfileIDRange = errors.New("profile ID is in reserved range")
)

func (c *Connection) CreateProfile(profile *Profile) error {
	if profile.ID == 0 {
		return c.pool.QueryRowContext(c.ctx, InsertProfile, profile.UserID, profile.GsbrCode).Scan(&profile.ID)
	}

	if profile.ID >= 1000000000 {
		return ErrReservedProfileIDRange
	}

	var exists bool
	err := c.pool.QueryRowContext(c.ctx, IsProfileIDInUse, profile.ID).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return ErrProfileIDInUse
	}

	_, err = c.pool.ExecContext(c.ctx, InsertProfileWithID, profile.ID, profile.UserID, profile.GsbrCode)
	return err
}

func (c *Connection) UpdateProfileID(profile *Profile, newProfileId uint32) error {
	if newProfileId >= 1000000000 {
		return ErrReservedProfileIDRange
	}

	var exists bool
	err := c.pool.QueryRowContext(c.ctx, IsProfileIDInUse, newProfileId).Scan(&exists)
	if err != nil {
		return err
	}

	if exists {
		return ErrProfileIDInUse
	}

	_, err = c.pool.ExecContext(c.ctx, UpdateUserProfileID, newProfileId, profile.UserID, profile.GsbrCode)
	if err == nil {
		profile.ID = newProfileId
	}

	return err
}

func (c *Connection) UpdateProfile(profile *Profile, data map[string]string) {
	firstName, firstNameExists := data["firstname"]
	lastName, lastNameExists := data["lastname"]

	_, err := c.pool.ExecContext(c.ctx, UpdateProfileTable, firstNameExists, firstName, lastNameExists, lastName, profile.ID)
	if err != nil {
		panic(err)
	}

	if firstNameExists {
		profile.FirstName = firstName
	}

	if lastNameExists {
		profile.LastName = lastName
	}
}

func (c *Connection) GetProfile(profileId uint32) (Profile, bool) {
	var profile Profile
	row := c.pool.QueryRowContext(c.ctx, GetProfile, profileId)
	err := row.Scan(&profile.UserID, &profile.GsbrCode, &profile.FirstName, &profile.LastName, &profile.LastIPAddress, &profile.LastInGameSn)
	if err != nil {
		return Profile{}, false
	}

	profile.ID = profileId
	return profile, true
}

func (c *Connection) ClearProfile(profileId uint32) (Profile, bool) {
	var profile Profile
	row := c.pool.QueryRowContext(c.ctx, ClearProfileQuery, profileId)
	err := row.Scan(&profile.UserID, &profile.GsbrCode, &profile.FirstName, &profile.LastName, &profile.LastIPAddress, &profile.LastInGameSn)

	if err != nil {
		return Profile{}, false
	}

	profile.ID = profileId
	return profile, true
}

func (c *Connection) BanProfile(profileId uint32, tos bool, length time.Duration, reason string, reasonHidden string, moderator string) bool {
	_, err := c.pool.ExecContext(c.ctx, UpdateProfileBan, time.Now().UTC(), time.Now().UTC().Add(length), reason, reasonHidden, moderator, tos, profileId)
	return err == nil
}

func (c *Connection) UnbanProfile(profileId uint32) bool {
	_, err := c.pool.ExecContext(c.ctx, DisableProfileBan, profileId)
	return err == nil
}

func (c *Connection) SearchProfileBan(profileId uint32, ipAddress string, lastIpAddress string) (
	tos bool, issued time.Time, expires time.Time, reason string, bannedProfileId uint32, gsbrCode string, inGameName string, err error) {
	row := c.pool.QueryRowContext(c.ctx, SearchProfileBanInfo, profileId, ipAddress, lastIpAddress)
	var hasBan bool
	err = row.Scan(&hasBan, &tos, &issued, &expires, &reason, &bannedProfileId, &gsbrCode, &inGameName)
	if err == nil && !hasBan {
		err = errors.New("no ban found")
	}
	if len(gsbrCode) > 4 {
		gsbrCode = gsbrCode[:4]
	}
	return tos, issued, expires, reason, bannedProfileId, gsbrCode, inGameName, err
}
