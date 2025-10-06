## Shell Completion

The CLI supports shell completion for instance names. After installing the completion scripts, use Tab to auto-complete instance names when connecting.

### Install completion

#### Bash

```bash
# Current session
source <(ssm completion bash)

# All sessions (Linux)
ssm completion bash > /etc/bash_completion.d/ssm

# All sessions (macOS with Homebrew)
ssm completion bash > /usr/local/etc/bash_completion.d/ssm
```

#### Zsh

```bash
# Enable completion if needed
echo "autoload -U compinit; compinit" >> ~/.zshrc

# Method 1 - fpath location
ssm completion zsh > "${fpath[1]}/_ssm"

# Method 2 - system-wide
sudo ssm completion zsh > /usr/local/share/zsh/site-functions/_ssm

# Method 3 - local functions directory
mkdir -p ~/.zsh/functions
ssm completion zsh > ~/.zsh/functions/_ssm
echo "fpath=(~/.zsh/functions $fpath)" >> ~/.zshrc

# Reload
exec zsh  # or: compinit
```

#### Fish

```bash
ssm completion fish > ~/.config/fish/completions/ssm.fish
```

#### PowerShell

```powershell
ssm completion powershell > ssm.ps1
# Add to your PowerShell profile
```

### Usage

```bash
ssm t-<Tab>    # Shows all instances starting with "t-"
ssm <Tab>      # Shows all available instance names
```
