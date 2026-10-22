#!/bin/bash
# Rebuild coder for native Termux: PIE build, TLS-align patch, stage to /sdcard.
# From Termux: proot-distro login ubuntu -- /root/build/build.sh
set -e
cd /root/build/coder
export PATH=/root/build/go/bin:$PATH GOPATH=/root/build/gopath GOCACHE=/root/build/gocache GOTOOLCHAIN=local
gofmt -l androidcompat cmd/coder
CGO_ENABLED=0 GOOS=linux GOARCH=arm64 go build -buildmode=pie -trimpath \
  -tags "slim,ts_omit_aws,ts_omit_bird,ts_omit_tap,ts_omit_kube" \
  -ldflags "-s -w -checklinkname=0 -X github.com/coder/coder/v2/buildinfo.tag=2.34.0" \
  -o /root/build/coder-pie ./cmd/coder
python3 - <<'PY'
import struct
src=open('/root/build/coder-pie','rb').read(); phoff=struct.unpack_from('<Q',src,32)[0]; phentsize,phnum=struct.unpack_from('<HH',src,54)
tls=[i for i in range(phnum) if struct.unpack_from('<I',src,phoff+i*phentsize)[0]==7][0]
a=bytearray(src); struct.pack_into('<Q',a,phoff+tls*phentsize+48,64); open('/root/build/coder-tls64','wb').write(a); print('tls patched')
PY
chmod +x /root/build/coder-tls64
cp /root/build/coder-tls64 /sdcard/Download/coder-native/coder
echo "staged sha256 prefix: $(sha256sum /sdcard/Download/coder-native/coder | cut -c1-16)"
