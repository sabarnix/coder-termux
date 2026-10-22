package main

import (
	"bufio"
	"context"
	"net"
	"os"
	"path/filepath"
	"strings"
	"sync/atomic"
	"time"

	"tailscale.com/net/interfaces"

	"github.com/coder/coder/v2/androidcompat"
)

// Bionic's dynamic linker can be invoked directly as "linker64 <exe> <args>",
// which is how Termux launches binaries from app data on Android 10+. The
// linker keeps the exe path in the kernel argument block and only hides it
// from Bionic libc. The Go runtime reads argv straight from the stack, so the
// executable path arrives as the first argument. When the running image is
// the linker, drop that entry so os.Args matches a normal launch. argv[0]
// itself is not a reliable signal: termux-exec preserves the caller's argv[0].
func init() {
	if len(os.Args) < 2 {
		return
	}
	exe, err := os.Readlink("/proc/self/exe")
	if err != nil {
		return
	}
	if strings.HasPrefix(filepath.Base(exe), "linker") {
		os.Args = os.Args[1:]
		// Android denies RTM_GETLINK to apps, which breaks Go's
		// net.Interfaces and with it tailnet startup.
		interfaces.RegisterInterfaceGetter(androidcompat.Interfaces)
	}
}

// Android has no /etc/resolv.conf, so Go's resolver falls back to localhost
// port 53 where nothing listens. Point the default resolver at the
// nameservers from Termux's resolv.conf when it exists, else at public ones.
func init() {
	if _, err := os.Stat("/etc/resolv.conf"); err == nil {
		return
	}
	servers := readNameservers(os.Getenv("PREFIX") + "/etc/resolv.conf")
	if len(servers) == 0 {
		servers = []string{"8.8.8.8:53", "1.1.1.1:53"}
	}
	var next atomic.Uint32
	net.DefaultResolver = &net.Resolver{
		PreferGo: true,
		Dial: func(ctx context.Context, network, _ string) (net.Conn, error) {
			i := next.Add(1) - 1
			d := net.Dialer{Timeout: 5 * time.Second}
			return d.DialContext(ctx, network, servers[int(i)%len(servers)])
		},
	}
}

func readNameservers(path string) []string {
	f, err := os.Open(path)
	if err != nil {
		return nil
	}
	defer f.Close()
	var servers []string
	sc := bufio.NewScanner(f)
	for sc.Scan() {
		fields := strings.Fields(sc.Text())
		if len(fields) >= 2 && fields[0] == "nameserver" {
			servers = append(servers, net.JoinHostPort(fields[1], "53"))
		}
	}
	return servers
}

// Go only searches Android's system certificate store when built for
// GOOS=android. This is a linux build running under Termux, so when no Linux
// CA bundle exists and the caller has not set SSL_CERT_FILE or SSL_CERT_DIR,
// point the root loader at Termux's CA bundle or Android's system store.
func init() {
	if os.Getenv("SSL_CERT_FILE") != "" || os.Getenv("SSL_CERT_DIR") != "" {
		return
	}
	for _, f := range []string{
		"/etc/ssl/certs/ca-certificates.crt",
		"/etc/pki/tls/certs/ca-bundle.crt",
		"/etc/ssl/ca-bundle.pem",
		"/etc/pki/tls/cacert.pem",
		"/etc/pki/ca-trust/extracted/pem/tls-ca-bundle.pem",
		"/etc/ssl/cert.pem",
	} {
		if _, err := os.Stat(f); err == nil {
			return
		}
	}
	if f := os.Getenv("PREFIX") + "/etc/tls/cert.pem"; os.Getenv("PREFIX") != "" {
		if _, err := os.Stat(f); err == nil {
			os.Setenv("SSL_CERT_FILE", f)
			return
		}
	}
	for _, d := range []string{"/apex/com.android.conscrypt/cacerts", "/system/etc/security/cacerts"} {
		if entries, err := os.ReadDir(d); err == nil && len(entries) > 0 {
			os.Setenv("SSL_CERT_DIR", d)
			return
		}
	}
}
