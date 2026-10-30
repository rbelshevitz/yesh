package main

import (
	"strings"
	"testing"
)

func TestIPAndCIDRValues(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "network.yesh"}
	address, err := r.eval("ip 192.0.2.42", 1)
	if err != nil || address.ip == nil || address.ip.String() != "192.0.2.42" {
		t.Fatalf("IP value: %#v, %v", address, err)
	}
	prefix, err := r.eval("cidr 192.0.2.17/24", 2)
	if err != nil || prefix.cidr == nil || prefix.cidr.String() != "192.0.2.17/24" {
		t.Fatalf("CIDR value: %#v, %v", prefix, err)
	}
	if _, err := r.eval("cidr 192.0.2.0/33", 3); err == nil {
		t.Fatal("expected invalid IPv4 prefix length")
	}
	if _, err := r.eval("cidr fe80::/64%eth0", 4); err == nil {
		t.Fatal("expected zone to be rejected in CIDR")
	}
}

func TestContainsIPInCIDR(t *testing.T) {
	items, err := parse("network = cidr 192.0.2.0/24\naddress = ip 192.0.2.42\nif contains $network $address:\n    echo inside\nelse:\n    echo outside\n", "network.yesh")
	if err != nil {
		t.Fatal(err)
	}
	r := &runtime{values: map[string]value{}, filename: "network.yesh"}
	if err := r.execute(items[:2]); err != nil {
		t.Fatal(err)
	}
	ok, err := r.evaluateCondition(items[2].(ifStmt).condition, 3)
	if err != nil || !ok {
		t.Fatalf("contains: ok=%v err=%v", ok, err)
	}

	items, err = parse("network = cidr 192.0.2.0/24\naddress = ip 2001:db8::1\nif contains $network $address:\n    echo bad\n", "mixed.yesh")
	if err != nil {
		t.Fatal(err)
	}
	r = &runtime{values: map[string]value{}, filename: "mixed.yesh"}
	if err := r.execute(items[:2]); err != nil {
		t.Fatal(err)
	}
	_, err = r.evaluateCondition(items[2].(ifStmt).condition, 3)
	if err == nil || !strings.Contains(err.Error(), "cannot mix IPv4 and IPv6") {
		t.Fatalf("expected family mismatch, got %v", err)
	}
}

func TestMaskedAndOverlapsCIDR(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "network.yesh"}
	masked, err := r.eval("masked cidr 192.0.2.17/24", 1)
	if err != nil || masked.cidr == nil || masked.cidr.String() != "192.0.2.0/24" {
		t.Fatalf("masked prefix: %#v, %v", masked, err)
	}
	items, err := parse("a = cidr 192.0.2.0/24\nb = cidr 192.0.2.128/25\nif overlaps $a $b:\n    echo overlap\n", "overlaps.yesh")
	if err != nil {
		t.Fatal(err)
	}
	r = &runtime{values: map[string]value{}, filename: "overlaps.yesh"}
	if err := r.execute(items[:2]); err != nil {
		t.Fatal(err)
	}
	ok, err := r.evaluateCondition(items[2].(ifStmt).condition, 3)
	if err != nil || !ok {
		t.Fatalf("overlaps: ok=%v err=%v", ok, err)
	}
}

func TestIPAddressClasses(t *testing.T) {
	for _, tc := range []struct {
		class, address string
		want           bool
	}{
		{"link-local", "ip fe80::1%eth0", true},
		{"multicast", "ip 224.0.0.1", true},
		{"global-unicast", "ip 192.0.2.1", true},
		{"loopback", "ip ::1", true},
	} {
		source := "address = " + tc.address + "\nif is " + tc.class + " $address:\n    echo yes\n"
		items, err := parse(source, "classes.yesh")
		if err != nil {
			t.Fatal(err)
		}
		r := &runtime{values: map[string]value{}, filename: "classes.yesh"}
		if err := r.execute(items[:1]); err != nil {
			t.Fatal(err)
		}
		got, err := r.evaluateCondition(items[1].(ifStmt).condition, 2)
		if err != nil || got != tc.want {
			t.Fatalf("class %s: got=%v err=%v", tc.class, got, err)
		}
	}
}

func TestCIDRFirstAndLast(t *testing.T) {
	r := &runtime{values: map[string]value{}, filename: "bounds.yesh"}
	first, err := r.eval("first cidr 192.0.2.17/31", 1)
	if err != nil || first.ip == nil || first.ip.String() != "192.0.2.16" {
		t.Fatalf("first: %#v, %v", first, err)
	}
	last, err := r.eval("last cidr 192.0.2.17/31", 2)
	if err != nil || last.ip == nil || last.ip.String() != "192.0.2.17" {
		t.Fatalf("last: %#v, %v", last, err)
	}
	ipv6, err := r.eval("last cidr 2001:db8::/126", 3)
	if err != nil || ipv6.ip == nil || ipv6.ip.String() != "2001:db8::3" {
		t.Fatalf("IPv6 last: %#v, %v", ipv6, err)
	}
}
