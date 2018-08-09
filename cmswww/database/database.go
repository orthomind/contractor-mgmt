// Copyright (c) 2017 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package database

import (
	"encoding/hex"
	"errors"

	"github.com/decred/politeia/politeiad/api/v1/identity"
)

var (
	// ErrUserNotFound indicates that a user name was not found in the
	// database.
	ErrUserNotFound = errors.New("user not found")

	// ErrUserExists indicates that a user already exists in the database.
	ErrUserExists = errors.New("user already exists")

	// ErrInvalidEmail indicates that a user's email is not properly formatted.
	ErrInvalidEmail = errors.New("invalid user email")

	// ErrShutdown is emitted when the database is shutting down.
	ErrShutdown = errors.New("database is shutting down")
)

// Identity wraps an ed25519 public key and timestamps to indicate if it is
// active.  If deactivated != 0 then the key is no longer valid.
type Identity struct {
	Key         [identity.PublicKeySize]byte // ed25519 public key
	Activated   int64                        // Time key as activated for use
	Deactivated int64                        // Time key was deactivated
}

// IsIdentityActive returns true if the identity is active, false otherwise
func IsIdentityActive(id Identity) bool {
	return id.Activated != 0 && id.Deactivated == 0
}

// ActiveIdentity returns a the current active key.  If there is no active
// valid key the call returns all 0s and false.
func ActiveIdentity(i []Identity) ([identity.PublicKeySize]byte, bool) {
	for _, v := range i {
		if IsIdentityActive(v) {
			return v.Key, true
		}
	}

	return [identity.PublicKeySize]byte{}, false
}

// ActiveIdentityString returns a string representation of the current active
// key.  If there is no active valid key the call returns all 0s and false.
func ActiveIdentityString(i []Identity) (string, bool) {
	key, ok := ActiveIdentity(i)
	return hex.EncodeToString(key[:]), ok
}

// User record.
type User struct {
	ID                               uint64 // Unique id
	Email                            string // Email address + lookup key.
	Username                         string // Unique username
	HashedPassword                   []byte // Blowfish hash
	Admin                            bool   // Is user an admin
	RegisterVerificationToken        []byte // Verification token during signup
	RegisterVerificationExpiry       int64  // Verification expiration
	UpdateIdentityVerificationToken  []byte // Verification token when creating a new identity
	UpdateIdentityVerificationExpiry int64  // Verification expiration
	LastLogin                        int64  // Unix timestamp of when the user last logged in
	FailedLoginAttempts              uint64 // Number of failed login a user has made in a row

	// All identities the user has ever used.  User should only have one
	// active key at a time.  We allow multiples in order to deal with key
	// loss.
	Identities []Identity
}

// Database interface that is required by the web server.
type Database interface {
	// User functions
	UserGet(string) (*User, error)           // Return user record, key is email
	UserGetByUsername(string) (*User, error) // Return user record given the username
	UserGetById(uint64) (*User, error)       // Return user record given its id
	UserNew(User) error                      // Add new user
	UserUpdate(User) error                   // Update existing user
	AllUsers(callbackFn func(u *User)) error // Iterate all users

	// Close performs cleanup of the backend.
	Close() error
}