import { define } from "gunshi";

// eslint-disable-next-line func-style
function getZshCompletion(): string {
  return `#compdef gdn

_gdn() {
  local line state
  local -a commands

  _arguments -C \\
    '1: :->command' \\
    '*:: :->args'

  case $state in
    command)
      commands=(repo wt complete)
      _describe 'command' commands
      ;;
    args)
      case $line[1] in
        repo)
          _gdn_repo
          ;;
        wt)
          _gdn_wt
          ;;
        complete)
          _values 'shell' zsh bash fish
          ;;
      esac
      ;;
  esac
}

_gdn_repo() {
  _arguments -C \\
    '1: :->subcmd' \\
    '*:: :->args'

  case $state in
    subcmd)
      _values 'repo command' list root clone migrate
      ;;
    args)
      case $line[1] in
        list)
          _arguments \\
            '--column[output columns]:columns:' \\
            '--sort[sort field]:field:' \\
            '--reverse[reverse sort]' \\
            '--limit[limit results]:n:' \\
            '--no-color[disable ANSI colors]' \\
            '--json[output as JSON]' \\
            '--include-worktrees[include linked worktrees]'
          ;;
        clone)
          _arguments \\
            '--shallow[shallow clone]' \\
            '--branch[clone specific branch]:branch:' \\
            '--submodules[clone submodules recursively]'
          ;;
        migrate)
          _arguments \\
            '--dry-run[dry run]' \\
            '--yes[skip confirmation]'
          ;;
      esac
      ;;
  esac
}

_gdn_wt() {
  _arguments -C \\
    '1: :->subcmd' \\
    '*:: :->args'

  case $state in
    subcmd)
      _values 'wt command' list root create switch delete prune migrate
      ;;
    args)
      case $line[1] in
        list|root|create|switch|delete|prune|migrate)
          _arguments \\
            '-C[target repository path]:path:_directories' \\
            '--dir[target repository path]:path:_directories'
          ;;
      esac

      case $line[1] in
        list)
          _arguments \\
            '--column[output columns]:columns:' \\
            '--sort[sort field]:field:' \\
            '--reverse[reverse sort]' \\
            '--limit[limit results]:n:' \\
            '--no-color[disable ANSI colors]' \\
            '--json[output as JSON]' \\
            '--include-branches[include non-worktree branches]'
          ;;
        create|switch)
          _arguments \\
            '--base[base branch]:branch:'
          ;;
        delete)
          _arguments \\
            '--force[force delete]'
          ;;
        prune|migrate)
          _arguments \\
            '--dry-run[dry run]' \\
            '--yes[skip confirmation]'
          ;;
      esac
      ;;
  esac
}

compdef _gdn gdn
`;
}

// eslint-disable-next-line func-style
function getBashCompletion(): string {
  const lines = [
    "_gdn() {",
    "  local cur prev words cword",
    "  _init_completion || return",
    "",
    "  case $cword in",
    "    1)",
    "      COMPREPLY=($(compgen -W \"repo wt complete\" -- \"$cur\"))",
    "      ;;",
    "    2)",
    '      case ${words[1]} in',
    "        repo)",
    "          COMPREPLY=($(compgen -W \"list root clone migrate\" -- \"$cur\"))",
    "          ;;",
    "        wt)",
    "          COMPREPLY=($(compgen -W \"list root create switch delete prune migrate\" -- \"$cur\"))",
    "          ;;",
    "        complete)",
    "          COMPREPLY=($(compgen -W \"zsh bash fish\" -- \"$cur\"))",
    "          ;;",
    "      esac",
    "      ;;",
    "  esac",
    "}",
    "",
    "complete -F _gdn gdn",
  ];
  return lines.join("\n");
}

// eslint-disable-next-line func-style
function getFishCompletion(): string {
  const lines = [
    "complete -c gdn -f",
    "",
    "# Top-level commands",
    "complete -c gdn -n '__fish_use_subcommand' -a repo -d 'Repository management'",
    "complete -c gdn -n '__fish_use_subcommand' -a wt -d 'Worktree management'",
    "complete -c gdn -n '__fish_use_subcommand' -a complete -d 'Generate shell completion'",
    "",
    "# repo subcommands",
    "complete -c gdn -n '__fish_seen_subcommand_from repo' -a list -d 'List managed repositories'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo' -a root -d 'Output gdn root path'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo' -a clone -d 'Clone a repository'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo' -a migrate -d 'Migrate a local repo'",
    "",
    "# repo list options",
    "complete -c gdn -n '__fish_seen_subcommand_from repo list' -l column -d 'Output columns'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo list' -l sort -d 'Sort field'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo list' -l reverse -d 'Reverse sort'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo list' -l limit -d 'Limit results'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo list' -l no-color -d 'Disable color'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo list' -l json -d 'JSON output'",
    "",
    "# repo clone options",
    "complete -c gdn -n '__fish_seen_subcommand_from repo clone' -l shallow -d 'Shallow clone'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo clone' -l branch -d 'Branch to clone'",
    "",
    "# repo migrate options",
    "complete -c gdn -n '__fish_seen_subcommand_from repo migrate' -l dry-run -d 'Dry run'",
    "complete -c gdn -n '__fish_seen_subcommand_from repo migrate' -l yes -d 'Skip confirmation'",
    "",
    "# wt subcommands",
    "complete -c gdn -n '__fish_seen_subcommand_from wt' -a list -d 'List worktrees'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt' -a root -d 'Output wt base directory'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt' -a create -d 'Create worktree'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt' -a switch -d 'Switch to worktree'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt' -a delete -d 'Delete worktree'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt' -a prune -d 'Prune merged worktrees'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt' -a migrate -d 'Migrate worktrees'",
    "",
    "# wt list options",
    "complete -c gdn -n '__fish_seen_subcommand_from wt list' -l column -d 'Output columns'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt list' -l no-color -d 'Disable color'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt list' -l json -d 'JSON output'",
    "",
    "# Common options for wt subcommands",
    "for cmd in list root create switch delete prune migrate",
    "  complete -c gdn -n \"__fish_seen_subcommand_from wt $cmd\" -s C -d 'Target repository path'",
    "end",
    "",
    "# wt delete options",
    "complete -c gdn -n '__fish_seen_subcommand_from wt delete' -l force -d 'Force delete'",
    "",
    "# wt prune/migrate options",
    "complete -c gdn -n '__fish_seen_subcommand_from wt prune' -l dry-run -d 'Dry run'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt prune' -l yes -d 'Skip confirmation'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt migrate' -l dry-run -d 'Dry run'",
    "complete -c gdn -n '__fish_seen_subcommand_from wt migrate' -l yes -d 'Skip confirmation'",
  ];
  return lines.join("\n");
}

const completions = {
  zsh: getZshCompletion,
  bash: getBashCompletion,
  fish: getFishCompletion,
} as const;

export const completeCommand = define({
  name: "complete",
  description: "シェル補完スクリプトを生成する",
  args: {
    shell: {
      type: "positional",
      required: true,
      choices: ["zsh", "bash", "fish"] as const,
      description: "シェルの種類 (zsh, bash, fish)",
    },
  },
  run: async (ctx) => {
    const { shell } = ctx.values as { shell: "zsh" | "bash" | "fish" };
    const script = completions[shell]();
    process.stdout.write(script);
  },
});
