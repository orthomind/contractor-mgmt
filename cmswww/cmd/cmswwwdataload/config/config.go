// Copyright (c) 2013-2014 The btcsuite developers
// Copyright (c) 2015-2018 The Decred developers
// Use of this source code is governed by an ISC
// license that can be found in the LICENSE file.

package config

import (
	"fmt"
	"os"
	"path/filepath"
	"strings"

	flags "github.com/btcsuite/go-flags"
	"github.com/decred/contractor-mgmt/cmswww/sharedconfig"
)

const (
	defaultDataDirname          = "dataload"
	defaultConfigFilename       = "cmswwwdataload.conf"
	defaultPoliteiadLogFilename = "politeiad.log"
	defaultCmswwwLogFilename    = "cmswww.log"
	defaultLogLevel             = "info"
)

var (
	defaultDataDir    = filepath.Join(sharedconfig.DefaultHomeDir, defaultDataDirname)
	defaultConfigFile = filepath.Join(defaultDataDir, defaultConfigFilename)
)

// config defines the configuration options for cmswwwdataload.
//
// See loadConfig for details on the configuration load process.
type Config struct {
	AdminEmail                  string `long:"adminemail" description:"Admin user email address"`
	AdminUser                   string `long:"adminuser" description:"Admin username"`
	AdminPass                   string `long:"adminpass" description:"Admin password"`
	ContractorEmail             string `long:"contractoremail" description:"Contractor user email address"`
	ContractorUser              string `long:"contractoruser" description:"Contractor user username"`
	ContractorPass              string `long:"contractorpass" description:"Contractor user password"`
	ContractorName              string `long:"contractorname" description:"Contractor user full name"`
	ContractorLocation          string `long:"contractorlocation" description:"Contractor user physical location"`
	ContractorExtendedPublicKey string `long:"contractorextendedpublickey" description:"Contractor extended public key"`
	Verbose                     bool   `short:"v" long:"verbose" description:"Verbose output"`
	DataDir                     string `long:"datadir" description:"Path to config/data directory"`
	ConfigFile                  string `long:"configfile" description:"Path to configuration file"`
	DebugLevel                  string `long:"debuglevel" description:"Logging level to use for servers {trace, debug, info, warn, error, critical}"`
	DeleteData                  bool   `long:"deletedata" description:"Delete all existing data from politeiad and cmswww before loading data"`
	IncludeTests                bool   `long:"includetests" description:"Includes running tests of different commands."`
	PoliteiadLogFile            string
	CmswwwLogFile               string
}

// cleanAndExpandPath expands environment variables and leading ~ in the
// passed path, cleans the result, and returns it.
func cleanAndExpandPath(path string) string {
	// Expand initial ~ to OS specific home directory.
	if strings.HasPrefix(path, "~") {
		homeDir := filepath.Dir(sharedconfig.DefaultHomeDir)
		path = strings.Replace(path, "~", homeDir, 1)
	}

	// NOTE: The os.ExpandEnv doesn't work with Windows-style %VARIABLE%,
	// but the variables can still be expanded via POSIX-style $VARIABLE.
	return filepath.Clean(os.ExpandEnv(path))
}

// filesExists reports whether the named file or directory exists.
func fileExists(name string) bool {
	if _, err := os.Stat(name); err != nil {
		if os.IsNotExist(err) {
			return false
		}
	}
	return true
}

// newConfigParser returns a new command line flags parser.
func newConfigParser(cfg *Config, options flags.Options) *flags.Parser {
	return flags.NewParser(cfg, options)
}

// Load initializes and parses the config using a config file and command
// line options.
//
// The configuration proceeds as follows:
// 	1) Start with a default config with sane settings
// 	2) Pre-parse the command line to check for an alternative config file
// 	3) Load configuration file overwriting defaults with any specified options
// 	4) Parse CLI options and overwrite/add any specified options
//
// The above results in rpc functioning properly without any config settings
// while still allowing the user to override settings with config files and
// command line options.  Command line options always take precedence.
func Load() (*Config, error) {
	// Default config.
	cfg := Config{
		AdminEmail:                  "admin@example.com",
		AdminUser:                   "admin",
		AdminPass:                   "password",
		ContractorEmail:             "contractor@example.com",
		ContractorUser:              "contractor",
		ContractorPass:              "password",
		ContractorName:              "John Smith",
		ContractorLocation:          "Dallas, TX, USA",
		ContractorExtendedPublicKey: "faketpub",
		DeleteData:                  false,
		Verbose:                     false,
		DataDir:                     defaultDataDir,
		ConfigFile:                  defaultConfigFile,
		DebugLevel:                  defaultLogLevel,
	}

	// Pre-parse the command line options to see if an alternative config
	// file or the version flag was specified.  Any errors aside from the
	// help message error can be ignored here since they will be caught by
	// the final parse below.
	preCfg := cfg
	preParser := newConfigParser(&preCfg, flags.HelpFlag)
	_, err := preParser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); ok && e.Type == flags.ErrHelp {
			fmt.Fprintln(os.Stderr, err)
			return nil, err
		}
	}

	// Show the version and exit if the version flag was specified.
	appName := filepath.Base(os.Args[0])
	appName = strings.TrimSuffix(appName, filepath.Ext(appName))
	usageMessage := fmt.Sprintf("Use %s -h to show usage", appName)

	// Update the data directory if specified. Since the data directory
	// is updated, other variables need to be updated to reflect the new changes.
	if preCfg.DataDir != "" {
		cfg.DataDir, _ = filepath.Abs(preCfg.DataDir)

		if preCfg.ConfigFile == defaultConfigFile {
			cfg.ConfigFile = filepath.Join(cfg.DataDir, defaultConfigFilename)
		} else {
			cfg.ConfigFile = cleanAndExpandPath(preCfg.ConfigFile)
		}
	}

	// Load additional config from file.
	var configFileError error
	parser := newConfigParser(&cfg, flags.Default)
	err = flags.NewIniParser(parser).ParseFile(cfg.ConfigFile)
	if err != nil {
		if _, ok := err.(*os.PathError); !ok {
			fmt.Fprintf(os.Stderr, "Error parsing config "+
				"file: %v\n", err)
			fmt.Fprintln(os.Stderr, usageMessage)
			return nil, err
		}
		configFileError = err
	}

	// Parse command line options again to ensure they take precedence.
	_, err = parser.Parse()
	if err != nil {
		if e, ok := err.(*flags.Error); !ok || e.Type != flags.ErrHelp {
			fmt.Fprintln(os.Stderr, usageMessage)
		}
		return nil, err
	}

	// Create the data directory if it doesn't already exist.
	funcName := "loadConfig"
	err = os.MkdirAll(cfg.DataDir, 0700)
	if err != nil {
		// Show a nicer error message if it's because a symlink is
		// linked to a directory that does not exist (probably because
		// it's not mounted).
		if e, ok := err.(*os.PathError); ok && os.IsExist(err) {
			if link, lerr := os.Readlink(e.Path); lerr == nil {
				str := "is symlink %s -> %s mounted?"
				err = fmt.Errorf(str, e.Path, link)
			}
		}

		str := "%s: Failed to create data directory: %v"
		err := fmt.Errorf(str, funcName, err)
		fmt.Fprintln(os.Stderr, err)
		return nil, err
	}

	if configFileError != nil {
		fmt.Printf("WARNING: %v\n", configFileError)
	}

	cfg.PoliteiadLogFile = filepath.Join(cfg.DataDir,
		defaultPoliteiadLogFilename)
	cfg.CmswwwLogFile = filepath.Join(cfg.DataDir,
		defaultCmswwwLogFilename)

	return &cfg, nil
}
