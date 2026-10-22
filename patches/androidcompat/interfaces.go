// Package androidcompat adapts a GOOS=linux build of the CLI to the
// restrictions Android places on apps, such as Termux, that run it.
package androidcompat

import (
	"bytes"
	"encoding/binary"
	"errors"
	"net"
	"os"
	"strings"
	"syscall"
	"unsafe"

	"golang.org/x/sys/unix"
	"tailscale.com/net/interfaces"
)

// Interfaces enumerates network interfaces for tailscale's interface getter
// hook. It prefers Go's native enumeration and falls back to an RTM_GETADDR
// based listing when that fails, as it does on Android, which denies
// RTM_GETLINK to apps targeting API level 30 and above.
func Interfaces() ([]interfaces.Interface, error) {
	ifs, err := net.Interfaces()
	if err == nil {
		ret := make([]interfaces.Interface, len(ifs))
		for i := range ifs {
			ret[i].Interface = &ifs[i]
		}
		return ret, nil
	}
	ret, addrsErr := interfacesFromAddrs()
	if addrsErr == nil {
		return ret, nil
	}
	ret, ioctlErr := interfacesFromIoctl()
	if ioctlErr != nil {
		return nil, errors.Join(err, addrsErr, ioctlErr)
	}
	return ret, nil
}

// interfacesFromAddrs lists addresses with RTM_GETADDR, which Android still
// permits, and resolves each interface's name, flags and MTU with SIOCGIF
// ioctls, mirroring Bionic's getifaddrs fallback. Interfaces that carry no
// address are not reported, and hardware addresses are left empty.
func interfacesFromAddrs() ([]interfaces.Interface, error) {
	tab, err := syscall.NetlinkRIB(syscall.RTM_GETADDR, syscall.AF_UNSPEC)
	if err != nil {
		return nil, os.NewSyscallError("netlinkrib", err)
	}
	msgs, err := syscall.ParseNetlinkMessage(tab)
	if err != nil {
		return nil, os.NewSyscallError("parsenetlinkmessage", err)
	}
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}
	defer unix.Close(fd)

	byIndex := make(map[int]*interfaces.Interface)
	var order []int
	for i := range msgs {
		m := &msgs[i]
		if m.Header.Type == syscall.NLMSG_DONE {
			break
		}
		if m.Header.Type != syscall.RTM_NEWADDR || len(m.Data) < syscall.SizeofIfAddrmsg {
			continue
		}
		ifam := (*syscall.IfAddrmsg)(unsafe.Pointer(&m.Data[0]))
		attrs, err := syscall.ParseNetlinkRouteAttr(m)
		if err != nil {
			return nil, os.NewSyscallError("parsenetlinkrouteattr", err)
		}
		idx := int(ifam.Index)
		ifi, ok := byIndex[idx]
		if !ok {
			ni, err := interfaceByIndex(fd, idx)
			if err != nil {
				// The interface disappeared between the two queries.
				continue
			}
			ifi = &interfaces.Interface{Interface: ni, AltAddrs: []net.Addr{}}
			byIndex[idx] = ifi
			order = append(order, idx)
		}
		if a := addrFromAttrs(ifam, attrs); a != nil {
			ifi.AltAddrs = append(ifi.AltAddrs, a)
		}
	}
	out := make([]interfaces.Interface, 0, len(order))
	for _, idx := range order {
		out = append(out, *byIndex[idx])
	}
	return out, nil
}

func interfaceByIndex(fd, idx int) (*net.Interface, error) {
	ifr, err := unix.NewIfreq("")
	if err != nil {
		return nil, err
	}
	ifr.SetUint32(uint32(idx))
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFNAME, ifr); err != nil {
		return nil, os.NewSyscallError("ioctl SIOCGIFNAME", err)
	}
	ni := &net.Interface{Index: idx, Name: ifr.Name()}
	ifr, err = unix.NewIfreq(ni.Name)
	if err != nil {
		return nil, err
	}
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, ifr); err == nil {
		ni.Flags = linkFlags(uint32(ifr.Uint16()))
	}
	if err := unix.IoctlIfreq(fd, unix.SIOCGIFMTU, ifr); err == nil {
		ni.MTU = int(ifr.Uint32())
	}
	return ni, nil
}

// addrFromAttrs mirrors the address selection in Go's net package: a
// point-to-point interface reports IFA_LOCAL, anything else IFA_ADDRESS.
func addrFromAttrs(ifam *syscall.IfAddrmsg, attrs []syscall.NetlinkRouteAttr) net.Addr {
	var local, address []byte
	for _, a := range attrs {
		switch a.Attr.Type {
		case syscall.IFA_LOCAL:
			local = a.Value
		case syscall.IFA_ADDRESS:
			address = a.Value
		}
	}
	raw := address
	if local != nil {
		raw = local
	}
	switch ifam.Family {
	case syscall.AF_INET:
		if len(raw) < net.IPv4len {
			return nil
		}
		return &net.IPNet{
			IP:   net.IPv4(raw[0], raw[1], raw[2], raw[3]),
			Mask: net.CIDRMask(int(ifam.Prefixlen), 8*net.IPv4len),
		}
	case syscall.AF_INET6:
		if len(raw) < net.IPv6len {
			return nil
		}
		ifa := &net.IPNet{
			IP:   make(net.IP, net.IPv6len),
			Mask: net.CIDRMask(int(ifam.Prefixlen), 8*net.IPv6len),
		}
		copy(ifa.IP, raw[:net.IPv6len])
		return ifa
	}
	return nil
}

func linkFlags(rawFlags uint32) net.Flags {
	var f net.Flags
	if rawFlags&syscall.IFF_UP != 0 {
		f |= net.FlagUp
	}
	if rawFlags&syscall.IFF_RUNNING != 0 {
		f |= net.FlagRunning
	}
	if rawFlags&syscall.IFF_BROADCAST != 0 {
		f |= net.FlagBroadcast
	}
	if rawFlags&syscall.IFF_LOOPBACK != 0 {
		f |= net.FlagLoopback
	}
	if rawFlags&syscall.IFF_POINTOPOINT != 0 {
		f |= net.FlagPointToPoint
	}
	if rawFlags&syscall.IFF_MULTICAST != 0 {
		f |= net.FlagMulticast
	}
	return f
}

// sizeofIfreq is sizeof(struct ifreq) on 64-bit Linux: a 16 byte name
// followed by a 24 byte union.
const sizeofIfreq = 40

// interfacesFromIoctl lists interfaces without any netlink socket, for
// hosts that deny netlink entirely. SIOCGIFCONF yields one entry per IPv4
// address, and SIOCGIF* ioctls supply index, flags, MTU and netmask. IPv6
// addresses cannot be discovered this way and are omitted.
func interfacesFromIoctl() ([]interfaces.Interface, error) {
	fd, err := unix.Socket(unix.AF_INET, unix.SOCK_DGRAM|unix.SOCK_CLOEXEC, 0)
	if err != nil {
		return nil, os.NewSyscallError("socket", err)
	}
	defer unix.Close(fd)

	var buf []byte
	for n := 64; ; n *= 2 {
		buf = make([]byte, n*sizeofIfreq)
		ifc := struct {
			Len int32
			_   [4]byte
			Buf *byte
		}{Len: int32(len(buf)), Buf: &buf[0]}
		if _, _, errno := unix.Syscall(unix.SYS_IOCTL, uintptr(fd), unix.SIOCGIFCONF, uintptr(unsafe.Pointer(&ifc))); errno != 0 {
			return nil, os.NewSyscallError("ioctl SIOCGIFCONF", errno)
		}
		if int(ifc.Len) < len(buf) {
			buf = buf[:ifc.Len]
			break
		}
	}

	byIndex := make(map[int]*interfaces.Interface)
	var order []int
	for off := 0; off+sizeofIfreq <= len(buf); off += sizeofIfreq {
		entry := buf[off : off+sizeofIfreq]
		name := string(bytes.TrimRight(entry[:unix.IFNAMSIZ], "\x00"))
		if name == "" || binary.NativeEndian.Uint16(entry[16:18]) != unix.AF_INET {
			continue
		}
		// Aliases such as "eth0:1" belong to the base interface.
		base, _, _ := strings.Cut(name, ":")
		ifr, err := unix.NewIfreq(base)
		if err != nil {
			continue
		}
		if err := unix.IoctlIfreq(fd, unix.SIOCGIFINDEX, ifr); err != nil {
			continue
		}
		idx := int(ifr.Uint32())
		ifi, ok := byIndex[idx]
		if !ok {
			ni := &net.Interface{Index: idx, Name: base}
			if err := unix.IoctlIfreq(fd, unix.SIOCGIFFLAGS, ifr); err == nil {
				ni.Flags = linkFlags(uint32(ifr.Uint16()))
			}
			if err := unix.IoctlIfreq(fd, unix.SIOCGIFMTU, ifr); err == nil {
				ni.MTU = int(ifr.Uint32())
			}
			ifi = &interfaces.Interface{Interface: ni, AltAddrs: []net.Addr{}}
			byIndex[idx] = ifi
			order = append(order, idx)
		}
		mask := net.CIDRMask(32, 32)
		if alias, err := unix.NewIfreq(name); err == nil {
			if err := unix.IoctlIfreq(fd, unix.SIOCGIFNETMASK, alias); err == nil {
				if m, err := alias.Inet4Addr(); err == nil {
					mask = net.IPv4Mask(m[0], m[1], m[2], m[3])
				}
			}
		}
		ifi.AltAddrs = append(ifi.AltAddrs, &net.IPNet{
			IP:   net.IPv4(entry[20], entry[21], entry[22], entry[23]),
			Mask: mask,
		})
	}
	out := make([]interfaces.Interface, 0, len(order))
	for _, idx := range order {
		out = append(out, *byIndex[idx])
	}
	return out, nil
}
