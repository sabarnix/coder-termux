package androidcompat

import (
	"net"
	"testing"

	"github.com/stretchr/testify/require"
)

// Self-consistency checks only. The fallback exists for hosts where Go's
// native enumeration fails or hangs, so it cannot be compared against
// net.Interfaces here. TestMain-free: run with -v to see the listing.
func TestInterfacesFromAddrs(t *testing.T) {
	t.Parallel()
	got, err := interfacesFromAddrs()
	require.NoError(t, err)
	require.NotEmpty(t, got)

	var sawLoopback bool
	seen := map[int]bool{}
	for _, ifi := range got {
		t.Logf("%d %s flags=%v mtu=%d addrs=%v", ifi.Index, ifi.Name, ifi.Flags, ifi.MTU, ifi.AltAddrs)
		require.False(t, seen[ifi.Index], "interface index %d reported twice", ifi.Index)
		seen[ifi.Index] = true
		require.NotEmpty(t, ifi.Name)
		require.Positive(t, ifi.MTU)
		require.NotEmpty(t, ifi.AltAddrs, "interface %s reported without addresses", ifi.Name)
		for _, a := range ifi.AltAddrs {
			ipn, ok := a.(*net.IPNet)
			require.True(t, ok, "address %v is not an IPNet", a)
			ones, bits := ipn.Mask.Size()
			require.NotZero(t, bits)
			require.LessOrEqual(t, ones, bits)
		}
		if ifi.Flags&net.FlagLoopback != 0 {
			sawLoopback = true
		}
	}
	require.True(t, sawLoopback, "loopback interface not reported")
}

// The ioctl listing must agree with the netlink listing on every IPv4
// address, interface index, flag set and MTU.
func TestInterfacesFromIoctlMatchesNetlink(t *testing.T) {
	t.Parallel()
	viaIoctl, err := interfacesFromIoctl()
	require.NoError(t, err)
	require.NotEmpty(t, viaIoctl)
	viaNetlink, err := interfacesFromAddrs()
	require.NoError(t, err)

	v4 := func(addrs []net.Addr) []string {
		var out []string
		for _, a := range addrs {
			if ipn, ok := a.(*net.IPNet); ok && ipn.IP.To4() != nil {
				out = append(out, ipn.String())
			}
		}
		return out
	}
	byIndex := map[int]int{}
	for i, ifi := range viaNetlink {
		byIndex[ifi.Index] = i
	}
	for _, ifi := range viaIoctl {
		t.Logf("%d %s flags=%v mtu=%d addrs=%v", ifi.Index, ifi.Name, ifi.Flags, ifi.MTU, ifi.AltAddrs)
		i, ok := byIndex[ifi.Index]
		require.True(t, ok, "interface %s (%d) missing from netlink listing", ifi.Name, ifi.Index)
		want := viaNetlink[i]
		require.Equal(t, want.Name, ifi.Name)
		require.Equal(t, want.Flags, ifi.Flags)
		require.Equal(t, want.MTU, ifi.MTU)
		require.ElementsMatch(t, v4(want.AltAddrs), v4(ifi.AltAddrs))
	}
	for _, want := range viaNetlink {
		if len(v4(want.AltAddrs)) == 0 {
			continue
		}
		found := false
		for _, ifi := range viaIoctl {
			found = found || ifi.Index == want.Index
		}
		require.True(t, found, "IPv4 interface %s missing from ioctl listing", want.Name)
	}
}
