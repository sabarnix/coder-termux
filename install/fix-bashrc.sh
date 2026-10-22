#!/data/data/com.termux/files/usr/bin/bash
# Fix the orphan `fi` left in ~/.bashrc after disabling an earlier proot
# auto-login block. Safe to re-run.
python3 - "$HOME/.bashrc" <<'PY'
import sys
p = sys.argv[1]
lines = open(p).read().split('\n')
out = []
prev = ''
for l in lines:
    s = l.strip()
    if s == 'fi' and prev.startswith('# disabled by'):
        l = '# disabled by install-claude.sh: fi'
    out.append(l)
    if s:
        prev = s
open(p, 'w').write('\n'.join(out))
PY
echo "--- ~/.bashrc:"; cat -n "$HOME/.bashrc"
bash -n "$HOME/.bashrc" && echo "syntax OK"
