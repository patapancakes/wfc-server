package database

import (
	"database/sql"
	"errors"
)

const (
	InsertUser          = `INSERT INTO users (id, unitcd, macadr, passwd, csnum) VALUES (?, ?, ?, ?, ?)`
	GetUser             = `SELECT unitcd, macadr, passwd, csnum FROM users WHERE id = ?`
	IsUserIDInUse       = `SELECT EXISTS(SELECT 1 FROM users WHERE id = ?)`
	IsMACInUse          = `SELECT EXISTS(SELECT 1 FROM users WHERE macadr = ?)`
	IsSerialNumberInUse = `SELECT EXISTS(SELECT 1 FROM users WHERE csnum = ?)`
)

type User struct {
	ID           uint64
	UnitCode     int
	MacAddress   string
	Password     int    // ds only
	SerialNumber string // wii only
}

func (u User) IsWii() bool {
	return u.UnitCode == 1
}

var (
	ErrUserIDInUse       = errors.New("user ID is already in use")
	ErrMACInUse          = errors.New("mac address is already in use")
	ErrSerialNumberInUse = errors.New("serial number is already in use")
)

func (c *Connection) CreateUser(user *User) error {
	var exists bool

	err := c.pool.QueryRowContext(c.ctx, IsUserIDInUse, user.ID).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrUserIDInUse
	}

	err = c.pool.QueryRowContext(c.ctx, IsMACInUse, user.MacAddress).Scan(&exists)
	if err != nil {
		return err
	}
	if exists {
		return ErrMACInUse
	}

	if user.IsWii() {
		err = c.pool.QueryRowContext(c.ctx, IsSerialNumberInUse, user.SerialNumber).Scan(&exists)
		if err != nil {
			return err
		}
		if exists {
			return ErrSerialNumberInUse
		}
	}

	var password *int
	var serial *string
	if !user.IsWii() {
		password = &user.Password
	} else {
		serial = &user.SerialNumber
	}

	_, err = c.pool.ExecContext(c.ctx, InsertUser, user.ID, user.UnitCode, user.MacAddress, password, serial)
	return err
}

func (c *Connection) GetUser(userId uint64) (User, bool) {
	var user User
	var password sql.NullInt16
	var serial sql.NullString
	err := c.pool.QueryRowContext(c.ctx, GetUser, userId).Scan(&user.UnitCode, &user.MacAddress, &password, &serial)
	if err != nil {
		return User{}, false
	}

	user.ID = userId
	user.Password = int(password.Int16)
	user.SerialNumber = serial.String

	return user, true
}
