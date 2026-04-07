package nut

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"testing"
	"time"
)

// fakeMatcher maps client command prefixes to server responses.
type fakeMatcher struct {
	prefix   string
	response string // multi-line response, lines joined by \n
}

// startFakeNUT starts a fake NUT server that replies to commands using matchers.
func startFakeNUT(t *testing.T, matchers []fakeMatcher) (host string, port int) {
	t.Helper()
	ln, err := net.Listen("tcp", "127.0.0.1:0")
	if err != nil {
		t.Fatalf("listen: %v", err)
	}
	t.Cleanup(func() { ln.Close() })
	go func() {
		conn, err := ln.Accept()
		if err != nil {
			return
		}
		defer conn.Close()
		sc := bufio.NewScanner(conn)
		w := bufio.NewWriter(conn)
		for sc.Scan() {
			line := sc.Text()
			matched := false
			for _, m := range matchers {
				if strings.HasPrefix(line, m.prefix) {
					fmt.Fprintln(w, m.response)
					w.Flush()
					matched = true
					break
				}
			}
			if !matched {
				fmt.Fprintln(w, "ERR UNKNOWN-COMMAND")
				w.Flush()
			}
		}
	}()
	addr := ln.Addr().String()
	h, ps, _ := net.SplitHostPort(addr)
	p := 0
	fmt.Sscanf(ps, "%d", &p)
	return h, p
}

func TestClientListUPS(t *testing.T) {
	host, port := startFakeNUT(t, []fakeMatcher{
		{
			prefix:   "LIST UPS",
			response: "BEGIN LIST UPS\nUPS myups \"Main UPS\"\nEND LIST UPS",
		},
	})
	c, err := Dial(host, port, 5*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	ups, err := c.ListUPS()
	if err != nil {
		t.Fatalf("ListUPS: %v", err)
	}
	if len(ups) != 1 {
		t.Fatalf("want 1 UPS, got %d", len(ups))
	}
	if ups[0].Name != "myups" {
		t.Errorf("want name=myups, got %q", ups[0].Name)
	}
	if ups[0].Description != "Main UPS" {
		t.Errorf("want desc=%q, got %q", "Main UPS", ups[0].Description)
	}
}

func TestClientListVar(t *testing.T) {
	host, port := startFakeNUT(t, []fakeMatcher{
		{
			prefix:   "LIST VAR",
			response: "BEGIN LIST VAR myups\nVAR myups ups.status \"OL\"\nVAR myups battery.charge \"100\"\nEND LIST VAR myups",
		},
	})
	c, err := Dial(host, port, 5*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	vars, err := c.ListVar("myups")
	if err != nil {
		t.Fatalf("ListVar: %v", err)
	}
	if len(vars) != 2 {
		t.Fatalf("want 2 vars, got %d", len(vars))
	}
	if vars[0].Name != "ups.status" || vars[0].Value != "OL" {
		t.Errorf("unexpected var[0]: %+v", vars[0])
	}
	if vars[1].Name != "battery.charge" || vars[1].Value != "100" {
		t.Errorf("unexpected var[1]: %+v", vars[1])
	}
}

func TestClientGetVar(t *testing.T) {
	host, port := startFakeNUT(t, []fakeMatcher{
		{
			prefix:   "GET VAR",
			response: `VAR myups ups.status "OL CHRG"`,
		},
	})
	c, err := Dial(host, port, 5*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	val, err := c.GetVar("myups", "ups.status")
	if err != nil {
		t.Fatalf("GetVar: %v", err)
	}
	if val != "OL CHRG" {
		t.Errorf("want %q, got %q", "OL CHRG", val)
	}
}

func TestClientNutError(t *testing.T) {
	host, port := startFakeNUT(t, []fakeMatcher{
		{prefix: "LIST UPS", response: "ERR ACCESS-DENIED"},
	})
	c, err := Dial(host, port, 5*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	_, err = c.ListUPS()
	if err == nil {
		t.Fatal("expected error")
	}
	if !IsAccessDenied(err) {
		t.Errorf("expected ACCESS-DENIED error, got %v", err)
	}
}

func TestUnquote(t *testing.T) {
	cases := []struct {
		in   string
		want string
	}{
		{`"hello"`, "hello"},
		{`"with \"quotes\""`, `with "quotes"`},
		{`"back\\slash"`, `back\slash`},
		{"plain", "plain"},
		{`""`, ""},
		{`  "trimmed"  `, "trimmed"},
	}
	for _, tc := range cases {
		got := unquote(tc.in)
		if got != tc.want {
			t.Errorf("unquote(%q) = %q, want %q", tc.in, got, tc.want)
		}
	}
}

func TestParseUPSLine(t *testing.T) {
	e := parseUPSLine(`UPS myups "A great UPS"`)
	if e.Name != "myups" || e.Description != "A great UPS" {
		t.Errorf("unexpected: %+v", e)
	}
}

func TestParseVarLine(t *testing.T) {
	e := parseVarLine(`VAR myups battery.charge "95"`)
	if e.Name != "battery.charge" || e.Value != "95" {
		t.Errorf("unexpected: %+v", e)
	}
}

func TestClientAuth(t *testing.T) {
	host, port := startFakeNUT(t, []fakeMatcher{
		{prefix: "USERNAME", response: "OK"},
		{prefix: "PASSWORD", response: "OK"},
		{prefix: "LIST UPS", response: "BEGIN LIST UPS\nEND LIST UPS"},
	})
	c, err := Dial(host, port, 5*time.Second)
	if err != nil {
		t.Fatalf("Dial: %v", err)
	}
	defer c.Close()

	if err := c.Auth("admin", "secret"); err != nil {
		t.Fatalf("Auth: %v", err)
	}
	ups, err := c.ListUPS()
	if err != nil {
		t.Fatalf("ListUPS after auth: %v", err)
	}
	if len(ups) != 0 {
		t.Errorf("want 0 ups, got %d", len(ups))
	}
}
