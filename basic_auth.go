package middleware

import (
	"crypto/subtle"
	"encoding/base64"
	http2 "net/http"
	"strings"

	"github.com/sujit-baniya/framework/contracts/http"
	"github.com/sujit-baniya/framework/utils"
)

// ConfigBasicAuth defines the config for middleware.
type ConfigBasicAuth struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c http.Context) bool

	// Users defines the allowed credentials
	//
	// Required. Default: map[string]string{}
	Users map[string]string

	// Realm is a string to define realm attribute of BasicAuth.
	// the realm identifies the system to authenticate against
	// and can be used by clients to save credentials
	//
	// Optional. Default: "Restricted".
	Realm string

	// Authorizer defines a function you can pass
	// to check the credentials however you want.
	// It will be called with a username and password
	// and is expected to return true or false to indicate
	// that the credentials were approved or not.
	//
	// Optional. Default: nil.
	Authorizer func(string, string) bool

	// Unauthorized defines the response body for unauthorized responses.
	// By default, it will return with a 401 Unauthorized and the correct WWW-Auth header
	//
	// Optional. Default: nil
	Unauthorized http.HandlerFunc

	// ContextUser is the key to store the username in Locals
	//
	// Optional. Default: "username"
	ContextUsername string

	// ContextPass is the key to store the password in Locals
	//
	// Optional. Default: "password"
	ContextPassword string
}

// ConfigBasicAuthDefault is the default config
var ConfigBasicAuthDefault = ConfigBasicAuth{
	Next:            nil,
	Users:           map[string]string{},
	Realm:           "Restricted",
	Authorizer:      nil,
	Unauthorized:    nil,
	ContextUsername: "username",
	ContextPassword: "password",
}

// Helper function to set default values
func configBasicAuthDefault(config ...ConfigBasicAuth) ConfigBasicAuth {
	// Return default config if nothing provided
	if len(config) < 1 {
		return ConfigBasicAuthDefault
	}

	// Override default config
	cfg := config[0]

	// Set default values
	if cfg.Next == nil {
		cfg.Next = ConfigBasicAuthDefault.Next
	}
	if cfg.Users == nil {
		cfg.Users = ConfigBasicAuthDefault.Users
	}
	if cfg.Realm == "" {
		cfg.Realm = ConfigBasicAuthDefault.Realm
	}
	if cfg.Authorizer == nil {
		cfg.Authorizer = func(user, pass string) bool {
			userPwd, exist := cfg.Users[user]
			return exist && subtle.ConstantTimeCompare(utils.UnsafeBytes(userPwd), utils.UnsafeBytes(pass)) == 1
		}
	}
	if cfg.Unauthorized == nil {
		cfg.Unauthorized = func(c http.Context) error {
			c.SetHeader("WWW-Authenticate", "basic realm="+cfg.Realm)
			c.AbortWithStatus(http2.StatusUnauthorized)
			return utils.ErrUnauthorized
		}
	}
	if cfg.ContextUsername == "" {
		cfg.ContextUsername = ConfigBasicAuthDefault.ContextUsername
	}
	if cfg.ContextPassword == "" {
		cfg.ContextPassword = ConfigBasicAuthDefault.ContextPassword
	}
	return cfg
}

func BasicAuth(config ConfigBasicAuth) http.HandlerFunc {
	// Set default config
	cfg := configBasicAuthDefault(config)
	return func(c http.Context) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get authorization header
		auth := c.Header("Authorization", "")

		// Check if the header contains content besides "basic".
		if len(auth) <= 6 || strings.ToLower(auth[:5]) != "basic" {
			return cfg.Unauthorized(c)
		}

		// Decode the header contents
		raw, err := base64.StdEncoding.DecodeString(auth[6:])
		if err != nil {
			return cfg.Unauthorized(c)
		}

		// Get the credentials
		creds := utils.UnsafeString(raw)

		// Check if the credentials are in the correct form
		// which is "username:password".
		index := strings.Index(creds, ":")
		if index == -1 {
			return cfg.Unauthorized(c)
		}

		// Get the username and password
		username := creds[:index]
		password := creds[index+1:]

		if cfg.Authorizer(username, password) {
			c.WithValue(cfg.ContextUsername, username)
			c.WithValue(cfg.ContextPassword, password)
			return c.Next()
		}

		// Authentication failed
		return cfg.Unauthorized(c)
	}
}
