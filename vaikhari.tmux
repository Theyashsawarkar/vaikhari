#!/usr/bin/env bash
# vaikhari as a tmux plugin: TPM entry point (also works with a plain
# `run-shell` on this file). Binds prefix + @vaikhari-key to open vaikhari
# (write a post for X and LinkedIn) in a popup; the same key inside the
# popup closes it, keeping what you wrote as a draft.
#
# Options (set them before TPM's `run` line):
#   @vaikhari-key       key after the prefix (default B; "none" binds nothing)
#   @vaikhari-width     popup width  (default 90%)
#   @vaikhari-height    popup height (default 88%)
#   @vaikhari-title     popup title  (default " vaikhari ")
#   @vaikhari-launcher  your own popup command instead of display-popup: it is
#                     run with display-popup's arguments appended (-E -d DIR
#                     -T TITLE CMD), tmux formats expanded, e.g.
#                     "~/.tmux/scripts/popup.sh '#{client_name}'"

dir=$(cd "$(dirname "${BASH_SOURCE[0]}")" && pwd)
run="$dir/scripts/vaikhari-run"

opt() { local v; v=$(tmux show-option -gqv "$1"); printf '%s' "${v:-$2}"; }
key=$(opt @vaikhari-key B)
[ "$key" = none ] && exit 0
width=$(opt @vaikhari-width 90%)
height=$(opt @vaikhari-height 88%)
title=$(opt @vaikhari-title ' vaikhari ')
launcher=$(opt @vaikhari-launcher '')

cmd="'$run' --popup '$key'"
if [ -n "$launcher" ]; then
  open="run -b \"$launcher -E -d '#{pane_current_path}' -T '$title' $cmd\""
else
  open="display-popup -E -w '$width' -h '$height' -d '#{pane_current_path}' -T '$title' \"$cmd\""
fi

tmux bind-key -N "Open vaikhari post popup (write once for X and LinkedIn; prefix $key again closes)" "$key" "$open"
