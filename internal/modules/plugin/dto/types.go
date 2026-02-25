package dto

type PluginInfo struct {
	Name         string
	Version      string
	Enabled      bool
	Binary       string
	Capabilities []string
}

type DoctorResult struct {
	Name            string
	ChecksumValid   bool
	BinaryReachable bool
	LifecycleOK     bool
	Error           string
}

type CommandInfo struct {
	ID              string
	Title           string
	Description     string
	Kind            string
	InputSchemaJSON string
	TimeoutMS       int
}

type ExecuteInput struct {
	PluginName string
	CommandID  string
	InputJSON  string
	SourceID   string
	SessionID  string
	VaultPath  string
	Cwd        string
	Env        map[string]string
}

type ExecuteOutput struct {
	PluginName string
	CommandID  string
	Stdout     string
	Stderr     string
	OutputJSON string
	ExitCode   int
}

type TTYPrepareInput struct {
	PluginName string
	CommandID  string
	InputJSON  string
	SourceID   string
	SessionID  string
	VaultPath  string
	Cwd        string
	Env        map[string]string
}

type TTYPrepareOutput struct {
	PluginName string
	CommandID  string
	Argv       []string
	Cwd        string
	Env        map[string]string
}
