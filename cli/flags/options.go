package flags

import (
	"fmt"
	"os"
	"path/filepath"

	"github.com/docker/cli/cli/config"
	"github.com/docker/cli/opts"
	"github.com/docker/go-connections/tlsconfig"
	"github.com/moby/moby/client"
	"github.com/sirupsen/logrus"
	"github.com/spf13/pflag"
)

const (
	// EnvEnableTLS is the name of the environment variable that can be used
	// to enable TLS for client connections. When set to a non-empty value, TLS
	// is enabled for API connections using TCP. For backward-compatibility, this
	// environment-variable can only be used to enable TLS, not to disable.
	//
	// Note that TLS is always enabled implicitly if the "--tls-verify" option
	// or "DOCKER_TLS_VERIFY" ([github.com/docker/docker/client.EnvTLSVerify])
	// env var is set to, which could be to either enable or disable TLS certification
	// validation. In both cases, TLS is enabled but, depending on the setting,
	// with verification disabled.
	EnvEnableTLS = "DOCKER_TLS"

	// DefaultCaFile is the default filename for the CA pem file
	DefaultCaFile = "ca.pem"
	// DefaultKeyFile is the default filename for the key pem file
	DefaultKeyFile = "key.pem"
	// DefaultCertFile is the default filename for the cert pem file
	DefaultCertFile = "cert.pem"
	// FlagTLSVerify is the flag name for the TLS verification option
	FlagTLSVerify = "tlsverify"
	// FormatHelp describes the --format flag behavior for list commands
	FormatHelp = `Format output using a custom template:
'table':            Print output in table format with column headers (default)
'table TEMPLATE':   Print output in table format using the given Go template
'json':             Print in JSON format
'TEMPLATE':         Print output using the given Go template.
Refer to https://docs.docker.com/go/formatting/ for more information about formatting output with templates`
	// InspectFormatHelp describes the --format flag behavior for inspect commands
	InspectFormatHelp = `Format output using a custom template:
'json':             Print in JSON format
'TEMPLATE':         Print output using the given Go template.
Refer to https://docs.docker.com/go/formatting/ for more information about formatting output with templates`
)

var (
	dockerCertPath  = os.Getenv(client.EnvOverrideCertPath)
	dockerTLSVerify = os.Getenv(client.EnvTLSVerify) != ""
	dockerTLS       = os.Getenv(EnvEnableTLS) != ""
)

// ClientOptions are the options used to configure the client cli.
type ClientOptions struct {
	Debug      bool
	Hosts      []string
	LogLevel   string
	TLS        bool
	TLSVerify  bool
	TLSOptions *tlsconfig.Options
	Context    string
	ConfigDir  string
}

// NewClientOptions returns a new ClientOptions.
func NewClientOptions() *ClientOptions {
	return &ClientOptions{}
}

// InstallFlags adds flags for the common options on the FlagSet
func (o *ClientOptions) InstallFlags(flags *pflag.FlagSet) {
	configDir := config.Dir()
	if dockerCertPath == "" {
		dockerCertPath = configDir
	}

	flags.StringVar(&o.ConfigDir, "config", configDir, "Location of client config files")
	flags.BoolVarP(&o.Debug, "debug", "D", false, "Enable debug mode")
	flags.StringVarP(&o.LogLevel, "log-level", "l", "info", `Set the logging level ("debug", "info", "warn", "error", "fatal")`)
	flags.BoolVar(&o.TLS, "tls", dockerTLS, "Use TLS; implied by --tlsverify")
	flags.BoolVar(&o.TLSVerify, FlagTLSVerify, dockerTLSVerify, "Use TLS and verify the remote")

	o.TLSOptions = &tlsconfig.Options{
		CAFile:   filepath.Join(dockerCertPath, DefaultCaFile),
		CertFile: filepath.Join(dockerCertPath, DefaultCertFile),
		KeyFile:  filepath.Join(dockerCertPath, DefaultKeyFile),
	}
	tlsOptions := o.TLSOptions
	flags.Var(opts.NewQuotedString(&tlsOptions.CAFile), "tlscacert", "Trust certs signed only by this CA")
	flags.Var(opts.NewQuotedString(&tlsOptions.CertFile), "tlscert", "Path to TLS certificate file")
	flags.Var(opts.NewQuotedString(&tlsOptions.KeyFile), "tlskey", "Path to TLS key file")

	// opts.ValidateHost is not used here, so as to allow connection helpers
	hostOpt := opts.NewNamedListOptsRef("hosts", &o.Hosts, nil)
	flags.VarP(hostOpt, "host", "H", "Daemon socket to connect to")
	flags.StringVarP(&o.Context, "context", "c", "",
		`Name of the context to use to connect to the daemon (overrides `+client.EnvOverrideHost+` env var and default context set with "docker context use")`)
}

// SetDefaultOptions sets default values for options after flag parsing is
// complete
func (o *ClientOptions) SetDefaultOptions(flags *pflag.FlagSet) {
	// Regardless of whether the user sets it to true or false, if they
	// specify --tlsverify at all then we need to turn on TLS
	// TLSVerify can be true even if not set due to DOCKER_TLS_VERIFY env var, so we need
	// to check that here as well
	if flags.Changed(FlagTLSVerify) || o.TLSVerify {
		o.TLS = true
	}

	if !o.TLS {
		o.TLSOptions = nil
	} else {
		tlsOptions := o.TLSOptions
		tlsOptions.InsecureSkipVerify = !o.TLSVerify

		// Reset CertFile and KeyFile to empty string if the user did not specify
		// the respective flags and the respective default files were not found.
		if !flags.Changed("tlscert") {
			if _, err := os.Stat(tlsOptions.CertFile); os.IsNotExist(err) {
				tlsOptions.CertFile = ""
			}
		}
		if !flags.Changed("tlskey") {
			if _, err := os.Stat(tlsOptions.KeyFile); os.IsNotExist(err) {
				tlsOptions.KeyFile = ""
			}
		}
	}
}

// SetLogLevel sets the logrus logging level
func SetLogLevel(logLevel string) {
	if logLevel != "" {
		lvl, err := logrus.ParseLevel(logLevel)
		if err != nil {
			_, _ = fmt.Fprintln(os.Stderr, "Unable to parse logging level:", logLevel)
			os.Exit(1)
		}
		logrus.SetLevel(lvl)
	} else {
		logrus.SetLevel(logrus.InfoLevel)
	}
}
