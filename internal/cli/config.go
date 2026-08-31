package cli

import "github.com/spf13/cobra"

func newConfigCmd() *cobra.Command {
	return &cobra.Command{
		Use:   "config",
		Short: "Print loaded config, provenance, and effective runtime values",
		Long: `Print the configuration in effect as TOML on stdout, followed by
commented provenance and effective-value sections. Nothing is modified
and no brew command runs.

Settings load in this order, and a later layer replaces a value an
earlier one sets: built-in defaults (dir = ".", profiles = [],
env_profiles = "", fail_fast = true), brewkit.toml, the environment
variable BREWKIT_PROFILES, then the flags --profiles and --profile, which
every command accepts. Only profiles has an environment variable and
flags; dir, env_profiles, and fail_fast come from the file alone. An
unknown key in brewkit.toml is an error, and dir must not be empty.

` + configDiscoveryHelp + `

Keys: dir (directory holding the profile files), profiles (list of
profile names), env_profiles (name of an environment variable whose
comma-separated value appends to profiles), and fail_fast (stop at the
first failing entry when true; attempt every entry and report all
failures at the end when false).

` + profileSelectionHelp + `

The TOML section reloads as brewkit.toml. The '# provenance:' block lists
each field as a commented, tab-separated path, value, and source, where
source is <default>, the path of brewkit.toml, <env>, or <pflag>, and
'# loaded_files = [...]' names the files read. The '# effective:' block
gives effective_dir (dir made absolute) and effective_profiles (the list
the apply commands use), with a note when local was appended. Nothing is
redacted; the configuration holds no secret fields. --quiet does not
suppress the report and --dry-run has no effect.`,
		RunE: func(cmd *cobra.Command, _ []string) error {
			return runConfig(cmd.Context())
		},
	}
}
