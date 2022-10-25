package middleware

import (
	"github.com/sujit-baniya/framework/utils"
	stdHttp "net/http"
	"strconv"
	"strings"

	"github.com/sujit-baniya/framework/contracts/http"
)

// ConfigCors defines the config for middleware.
type ConfigCors struct {
	// Next defines a function to skip this middleware when returned true.
	//
	// Optional. Default: nil
	Next func(c http.Context) bool

	// AllowOrigin defines a list of origins that may access the resource.
	//
	// Optional. Default value "*"
	AllowOrigins string

	// AllowMethods defines a list methods allowed when accessing the resource.
	// This is used in response to a preflight request.
	//
	// Optional. Default value "GET,POST,HEAD,PUT,DELETE,PATCH"
	AllowMethods string

	// AllowHeaders defines a list of request headers that can be used when
	// making the actual request. This is in response to a preflight request.
	//
	// Optional. Default value "".
	AllowHeaders string

	// AllowCredentials indicates whether or not the response to the request
	// can be exposed when the credentials flag is true. When used as part of
	// a response to a preflight request, this indicates whether or not the
	// actual request can be made using credentials.
	//
	// Optional. Default value false.
	AllowCredentials bool

	// ExposeHeaders defines a whitelist headers that clients are allowed to
	// access.
	//
	// Optional. Default value "".
	ExposeHeaders string

	// MaxAge indicates how long (in seconds) the results of a preflight request
	// can be cached.
	//
	// Optional. Default value 0.
	MaxAge int
}

// ConfigCorsDefault is the default config
var ConfigCorsDefault = ConfigCors{
	Next:         nil,
	AllowOrigins: "*",
	AllowMethods: strings.Join([]string{
		utils.MethodGet,
		utils.MethodPost,
		utils.MethodHead,
		utils.MethodPut,
		utils.MethodDelete,
		utils.MethodPatch,
	}, ","),
	AllowHeaders:     "",
	AllowCredentials: false,
	ExposeHeaders:    "",
	MaxAge:           0,
}

// Cors creates a new middleware handler
func Cors(config ...ConfigCors) http.HandlerFunc {
	// Set default config
	cfg := ConfigCorsDefault

	// Override config if provided
	if len(config) > 0 {
		cfg = config[0]

		// Set default values
		if cfg.AllowMethods == "" {
			cfg.AllowMethods = ConfigCorsDefault.AllowMethods
		}
		if cfg.AllowOrigins == "" {
			cfg.AllowOrigins = ConfigCorsDefault.AllowOrigins
		}
	}

	// Convert string to slice
	allowOrigins := strings.Split(strings.ReplaceAll(cfg.AllowOrigins, " ", ""), ",")

	// Strip white spaces
	allowMethods := strings.ReplaceAll(cfg.AllowMethods, " ", "")
	allowHeaders := strings.ReplaceAll(cfg.AllowHeaders, " ", "")
	exposeHeaders := strings.ReplaceAll(cfg.ExposeHeaders, " ", "")

	// Convert int to string
	maxAge := strconv.Itoa(cfg.MaxAge)

	// Return new handler
	return func(c http.Context) error {
		// Don't execute middleware if Next returns true
		if cfg.Next != nil && cfg.Next(c) {
			return c.Next()
		}

		// Get origin header
		origin := c.Header(utils.HeaderOrigin, "")
		allowOrigin := ""

		// Check allowed origins
		for _, o := range allowOrigins {
			if o == "*" && cfg.AllowCredentials {
				allowOrigin = origin
				break
			}
			if o == "*" || o == origin {
				allowOrigin = o
				break
			}
			if matchSubdomain(origin, o) {
				allowOrigin = origin
				break
			}
		}

		// Simple request
		if c.Method() != stdHttp.MethodOptions {
			c.Vary(utils.HeaderOrigin)
			c.SetHeader(utils.HeaderAccessControlAllowOrigin, allowOrigin)

			if cfg.AllowCredentials {
				c.SetHeader(utils.HeaderAccessControlAllowCredentials, "true")
			}
			if exposeHeaders != "" {
				c.SetHeader(utils.HeaderAccessControlExposeHeaders, exposeHeaders)
			}
			return c.Next()
		}

		// Preflight request
		c.Vary(utils.HeaderOrigin)
		c.Vary(utils.HeaderAccessControlRequestMethod)
		c.Vary(utils.HeaderAccessControlRequestHeaders)
		c.SetHeader(utils.HeaderAccessControlAllowOrigin, allowOrigin)
		c.SetHeader(utils.HeaderAccessControlAllowMethods, allowMethods)

		// Set Allow-Credentials if set to true
		if cfg.AllowCredentials {
			c.SetHeader(utils.HeaderAccessControlAllowCredentials, "true")
		}

		// Set Allow-Headers if not empty
		if allowHeaders != "" {
			c.SetHeader(utils.HeaderAccessControlAllowHeaders, allowHeaders)
		} else {
			h := c.Header(utils.HeaderAccessControlRequestHeaders, "")
			if h != "" {
				c.SetHeader(utils.HeaderAccessControlAllowHeaders, h)
			}
		}

		// Set MaxAge is set
		if cfg.MaxAge > 0 {
			c.SetHeader(utils.HeaderAccessControlMaxAge, maxAge)
		}

		// Send 204 No Content
		return c.String(utils.StatusNoContent, "")
	}
}

func matchScheme(domain, pattern string) bool {
	didx := strings.Index(domain, ":")
	pidx := strings.Index(pattern, ":")
	return didx != -1 && pidx != -1 && domain[:didx] == pattern[:pidx]
}

// matchSubdomain compares authority with wildcard
func matchSubdomain(domain, pattern string) bool {
	if !matchScheme(domain, pattern) {
		return false
	}
	didx := strings.Index(domain, "://")
	pidx := strings.Index(pattern, "://")
	if didx == -1 || pidx == -1 {
		return false
	}
	domAuth := domain[didx+3:]
	// to avoid long loop by invalid long domain
	if len(domAuth) > 253 {
		return false
	}
	patAuth := pattern[pidx+3:]

	domComp := strings.Split(domAuth, ".")
	patComp := strings.Split(patAuth, ".")
	for i := len(domComp)/2 - 1; i >= 0; i-- {
		opp := len(domComp) - 1 - i
		domComp[i], domComp[opp] = domComp[opp], domComp[i]
	}
	for i := len(patComp)/2 - 1; i >= 0; i-- {
		opp := len(patComp) - 1 - i
		patComp[i], patComp[opp] = patComp[opp], patComp[i]
	}

	for i, v := range domComp {
		if len(patComp) <= i {
			return false
		}
		p := patComp[i]
		if p == "*" {
			return true
		}
		if p != v {
			return false
		}
	}
	return false
}
