package engine

import "time"

// TraceStatus is the result of correlating a trace against the app registry.
type TraceStatus string

const (
	StatusOwned     TraceStatus = "owned"
	StatusOrphaned  TraceStatus = "orphaned"
	StatusUnknown   TraceStatus = "unknown"
	StatusDeadAgent TraceStatus = "dead_agent"
)

// RiskLevel drives the safety gate for deletion.
type RiskLevel string

const (
	RiskGreen  RiskLevel = "green"
	RiskYellow RiskLevel = "yellow"
	RiskRed    RiskLevel = "red"
)

// InstallSource is where an app or tool came from.
type InstallSource string

const (
	SourceApplicationsDir InstallSource = "applications"
	SourceBrew            InstallSource = "brew"
	SourceBrewCask        InstallSource = "brew-cask"
	SourceMAS             InstallSource = "mas"
	SourceNPM             InstallSource = "npm"
	SourcePip             InstallSource = "pip"
	SourceCargo           InstallSource = "cargo"
	SourceMCP             InstallSource = "mcp"
	SourceLaunchAgent     InstallSource = "launchagent"
	SourceBinary          InstallSource = "binary"
)

// AppEntry is one installed application or tool in the registry.
type AppEntry struct {
	Name         string
	BundleID     string
	Version      string
	Source       InstallSource
	InstallPath  string
	LastLaunch   time.Time
	SizeBytes    int64
}

// Trace is one file or directory found during scanning.
type Trace struct {
	Path        string
	SizeBytes   int64
	ModTime     time.Time
	Status      TraceStatus
	OwnerApp    string        // populated when Status == StatusOwned
	Risk        RiskLevel
	Category    string        // "orphaned_trace", "dead_agent", "unknown_install", "dev_cache"
}

// ScanResult is the full output of a scan session.
type ScanResult struct {
	Orphans    []Trace
	DeadAgents []Trace
	Unknowns   []Trace
	DevCaches  []Trace
	UnusedApps []AppEntry
}
