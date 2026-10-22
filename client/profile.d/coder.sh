# Source this from ~/.bashrc.
#   . /path/to/coder-termux/client/profile.d/coder.sh
# Set CODER_WORKSPACE to the workspace name you connect to most, then `wrk`
# is a one-key shortcut that attaches to its persistent herdr session.
: "${CODER_WORKSPACE:=workspace}"
# shellcheck disable=SC2139
alias wrk='ssh -t '"$CODER_WORKSPACE"'.coder /home/coder/.local/bin/herdr'
# shellcheck disable=SC2139
alias mwrk='mosh-coder '"$CODER_WORKSPACE"
