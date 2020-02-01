package db

import (
	"net/url"
	"strconv"
	"strings"
	"time"

	"github.com/blend/go-sdk/env"
	"github.com/blend/go-sdk/ex"
	"github.com/blend/go-sdk/stringutil"
)

// NewConfigFromDSN creates a new config from a dsn.
func NewConfigFromDSN(dsn string) (*Config, error) {
	parsed, err := ParseURL(dsn)
	if err != nil {
		return nil, ex.New(err)
	}

	var config Config
	pieces := stringutil.SplitSpace(parsed)
	for _, piece := range pieces {
		if strings.HasPrefix(piece, "host=") {
			config.Host = strings.TrimPrefix(piece, "host=")
		} else if strings.HasPrefix(piece, "port=") {
			config.Port = strings.TrimPrefix(piece, "port=")
		} else if strings.HasPrefix(piece, "dbname=") {
			config.Database = strings.TrimPrefix(piece, "dbname=")
		} else if strings.HasPrefix(piece, "user=") {
			config.Username = strings.TrimPrefix(piece, "user=")
		} else if strings.HasPrefix(piece, "password=") {
			config.Password = strings.TrimPrefix(piece, "password=")
		} else if strings.HasPrefix(piece, "sslmode=") {
			config.SSLMode = strings.TrimPrefix(piece, "sslmode=")
		} else if strings.HasPrefix(piece, "search_path=") {
			config.Schema = strings.TrimPrefix(piece, "search_path=")
		} else if strings.HasPrefix(piece, "connect_timeout=") {
			config.ConnectTimeout, err = strconv.Atoi(strings.TrimPrefix(piece, "connect_timeout="))
			if err != nil {
				return nil, ex.New(err, ex.OptMessage("field: connect_timeout"))
			}
		}
	}

	return &config, nil
}

// NewConfigFromEnv returns a new config from the environment.
// The environment variable mappings are as follows:
//	-	DATABSE_URL 	= DSN 	//note that this has precedence over other vars (!!)
// 	-	DB_HOST 		= Host
//	-	DB_PORT 		= Port
//	- 	DB_NAME 		= Database
//	-	DB_SCHEMA		= Schema
//	-	DB_USER 		= Username
//	-	DB_PASSWORD 	= Password
//	-	DB_SSLMODE 		= SSLMode
func NewConfigFromEnv() (config Config, err error) {
	if err = env.Env().ReadInto(&config); err != nil {
		return
	}
	return
}

// MustNewConfigFromEnv returns a new config from the environment,
// it will panic if there is an error.
func MustNewConfigFromEnv() Config {
	cfg, err := NewConfigFromEnv()
	if err != nil {
		panic(err)
	}
	return cfg
}

// Config is a set of connection config options.
type Config struct {
	// Engine is the database engine.
	Engine string `json:"engine,omitempty" yaml:"engine,omitempty" env:"DB_ENGINE"`
	// DSN is a fully formed DSN (this skips DSN formation from all other variables outside `schema`).
	DSN string `json:"dsn,omitempty" yaml:"dsn,omitempty" env:"DATABASE_URL"`
	// Host is the server to connect to.
	Host string `json:"host,omitempty" yaml:"host,omitempty" env:"DB_HOST"`
	// Port is the port to connect to.
	Port string `json:"port,omitempty" yaml:"port,omitempty" env:"DB_PORT"`
	// DBName is the database name
	Database string `json:"database,omitempty" yaml:"database,omitempty" env:"DB_NAME"`
	// Schema is the application schema within the database, defaults to `public`. This schema is used to set the
	// Postgres "search_path" If you want to reference tables in other schemas, you'll need to specify those schemas
	// in your queries e.g. "SELECT * FROM schema_two.table_one..."
	// Using the public schema in a production application is considered bad practice as newly created roles will have
	// visibility into this data by default. We strongly recommend specifying this option and using a schema that is
	// owned by your service's role
	// We recommend against setting a multi-schema search_path, but if you really want to, you provide multiple comma-
	// separated schema names as the value for this config, or you can dbc.Invoke().Exec a SET statement on a newly
	// opened connection such as "SET search_path = 'schema_one,schema_two';" Again, we recommend against this practice
	// and encourage you to specify schema names beyond the first in your queries.
	Schema string `json:"schema,omitempty" yaml:"schema,omitempty" env:"DB_SCHEMA"`
	// Username is the username for the connection via password auth.
	Username string `json:"username,omitempty" yaml:"username,omitempty" env:"DB_USER"`
	// Password is the password for the connection via password auth.
	Password string `json:"password,omitempty" yaml:"password,omitempty" env:"DB_PASSWORD"`
	// ConnectTimeout is the connection timeout in seconds.
	ConnectTimeout int `json:"connectTimeout" yaml:"connectTimeout" env:"DB_CONNECT_TIMEOUT"`
	// SSLMode is the sslmode for the connection.
	SSLMode string `json:"sslMode,omitempty" yaml:"sslMode,omitempty" env:"DB_SSLMODE"`
	// PlanCacheDisabled indicates if we should use the prepared statement plan cache.
	PlanCacheDisabled bool `json:"planCacheDisabled,omitempty" yaml:"planCacheDisabled,omitempty" env:"DB_DISABLE_PLAN_CACHE"`
	// IdleConnections is the number of idle connections.
	IdleConnections int `json:"idleConnections,omitempty" yaml:"idleConnections,omitempty" env:"DB_IDLE_CONNECTIONS"`
	// MaxConnections is the maximum number of connections.
	MaxConnections int `json:"maxConnections,omitempty" yaml:"maxConnections,omitempty" env:"DB_MAX_CONNECTIONS"`
	// MaxLifetime is the maximum time a connection can be open.
	MaxLifetime time.Duration `json:"maxLifetime,omitempty" yaml:"maxLifetime,omitempty" env:"DB_MAX_LIFETIME"`
	// BufferPoolSize is the number of query composition buffers to maintain.
	BufferPoolSize int `json:"bufferPoolSize,omitempty" yaml:"bufferPoolSize,omitempty" env:"DB_BUFFER_POOL_SIZE"`
}

// IsZero returns if the config is unset.
func (c Config) IsZero() bool {
	return c.DSN == "" && c.Host == "" && c.Port == "" && c.Database == "" && c.Schema == "" && c.Username == "" && c.Password == "" && c.SSLMode == ""
}

// EngineOrDefault returns the database engine.
func (c Config) EngineOrDefault() string {
	if c.Engine != "" {
		return c.Engine
	}
	return DefaultEngine
}

// HostOrDefault returns the postgres host for the connection or a default.
func (c Config) HostOrDefault() string {
	if c.Host != "" {
		return c.Host
	}
	return DefaultHost
}

// PortOrDefault returns the port for a connection if it is not the standard postgres port.
func (c Config) PortOrDefault() string {
	if c.Port != "" {
		return c.Port
	}
	return DefaultPort
}

// DatabaseOrDefault returns the connection database or a default.
func (c Config) DatabaseOrDefault(inherited ...string) string {
	if c.Database != "" {
		return c.Database
	}
	return DefaultDatabase
}

// SchemaOrDefault returns the schema on the search_path or the default ("public"). It's considered bad practice to
// use the public schema in production
func (c Config) SchemaOrDefault(inherited ...string) string {
	if c.Schema != "" {
		return c.Schema
	}
	return DefaultSchema
}

// IdleConnectionsOrDefault returns the number of idle connections or a default.
func (c Config) IdleConnectionsOrDefault(inherited ...int) int {
	if c.IdleConnections > 0 {
		return c.IdleConnections
	}
	return DefaultIdleConnections
}

// MaxConnectionsOrDefault returns the maximum number of connections or a default.
func (c Config) MaxConnectionsOrDefault(inherited ...int) int {
	if c.MaxConnections > 0 {
		return c.MaxConnections
	}
	return DefaultMaxConnections
}

// MaxLifetimeOrDefault returns the maximum lifetime of a driver connection.
func (c Config) MaxLifetimeOrDefault() time.Duration {
	if c.MaxLifetime > 0 {
		return c.MaxLifetime
	}
	return DefaultMaxLifetime
}

// BufferPoolSizeOrDefault returns the number of query buffers to maintain or a default.
func (c Config) BufferPoolSizeOrDefault() int {
	if c.BufferPoolSize > 0 {
		return c.BufferPoolSize
	}
	return DefaultBufferPoolSize
}

// CreateDSN creates a postgres connection string from the config.
func (c Config) CreateDSN() string {
	if c.DSN != "" {
		return c.DSN
	}

	host := c.HostOrDefault()
	if c.PortOrDefault() != "" {
		host = host + ":" + c.PortOrDefault()
	}

	dsn := &url.URL{
		Scheme: "postgres",
		Host:   host,
		Path:   c.DatabaseOrDefault(),
	}

	if len(c.Username) > 0 {
		if len(c.Password) > 0 {
			dsn.User = url.UserPassword(c.Username, c.Password)
		} else {
			dsn.User = url.User(c.Username)
		}
	}

	queryArgs := url.Values{}
	if len(c.SSLMode) > 0 {
		queryArgs.Add("sslmode", c.SSLMode)
	}
	if c.ConnectTimeout > 0 {
		queryArgs.Add("connect_timeout", strconv.Itoa(c.ConnectTimeout))
	}
	if c.Schema != "" {
		queryArgs.Add("search_path", c.Schema)
	}

	dsn.RawQuery = queryArgs.Encode()
	return dsn.String()
}

// Resolve creates a DSN and reparses it, in case some values need to be coalesced.
func (c Config) Resolve() (*Config, error) {
	return NewConfigFromDSN(c.CreateDSN())
}

// MustResolve creates a DSN and reparses it, in case some values need to be coalesced,
// and panics if there is an error.
func (c Config) MustResolve() *Config {
	cfg, err := NewConfigFromDSN(c.CreateDSN())
	if err != nil {
		panic(err)
	}
	return cfg
}

// ValidateProduction validates production configuration for the config.
func (c Config) ValidateProduction() error {
	if !(len(c.SSLMode) == 0 ||
		stringutil.EqualsCaseless(c.SSLMode, SSLModeRequire) ||
		stringutil.EqualsCaseless(c.SSLMode, SSLModeVerifyCA) ||
		stringutil.EqualsCaseless(c.SSLMode, SSLModeVerifyFull)) {
		return ex.New(ErrUnsafeSSLMode, ex.OptMessagef("sslmode: %s", c.SSLMode))
	}

	if len(c.Username) == 0 {
		return ex.New(ErrUsernameUnset)
	}
	if len(c.Password) == 0 {
		return ex.New(ErrPasswordUnset)
	}
	return nil
}
