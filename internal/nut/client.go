// Package nut implements a client for the Network UPS Tools (NUT) protocol.
// The NUT protocol is a simple line-based text protocol over TCP (default port 3493).
package nut

import (
	"bufio"
	"fmt"
	"net"
	"strings"
	"time"
)

// Client is a synchronous NUT protocol client for a single TCP connection.
type Client struct {
	conn    net.Conn
	scanner *bufio.Scanner
	writer  *bufio.Writer
}

// Dial opens a TCP connection to a NUT server.
func Dial(addr string, port int, timeout time.Duration) (*Client, error) {
	address := fmt.Sprintf("%s:%d", addr, port)
	conn, err := net.DialTimeout("tcp", address, timeout)
	if err != nil {
		return nil, fmt.Errorf("nut: dial %s: %w", address, err)
	}
	conn.SetDeadline(time.Now().Add(timeout)) //nolint:errcheck
	sc := bufio.NewScanner(conn)
	sc.Buffer(make([]byte, 64*1024), 64*1024)
	return &Client{
		conn:    conn,
		scanner: sc,
		writer:  bufio.NewWriter(conn),
	}, nil
}

// Close closes the connection.
func (c *Client) Close() error {
	_ = c.sendLine("LOGOUT")
	return c.conn.Close()
}

func (c *Client) sendLine(line string) error {
	if _, err := fmt.Fprintf(c.writer, "%s\n", line); err != nil {
		return err
	}
	return c.writer.Flush()
}

func (c *Client) readLine() (string, error) {
	if !c.scanner.Scan() {
		if err := c.scanner.Err(); err != nil {
			return "", err
		}
		return "", fmt.Errorf("nut: connection closed")
	}
	line := c.scanner.Text()
	if strings.HasPrefix(line, "ERR ") {
		return "", &NutError{Code: strings.TrimPrefix(line, "ERR ")}
	}
	return line, nil
}

// Auth authenticates with the NUT server using username and password.
func (c *Client) Auth(username, password string) error {
	if err := c.sendLine("USERNAME " + username); err != nil {
		return err
	}
	if _, err := c.readLine(); err != nil {
		return fmt.Errorf("nut: USERNAME failed: %w", err)
	}
	if err := c.sendLine("PASSWORD " + password); err != nil {
		return err
	}
	if _, err := c.readLine(); err != nil {
		return fmt.Errorf("nut: PASSWORD failed: %w", err)
	}
	return nil
}

// GetVer returns the NUT daemon version string.
func (c *Client) GetVer() (string, error) {
	if err := c.sendLine("VER"); err != nil {
		return "", err
	}
	return c.readLine()
}

// GetNetVer returns the NUT network protocol version.
func (c *Client) GetNetVer() (string, error) {
	if err := c.sendLine("NETVER"); err != nil {
		return "", err
	}
	return c.readLine()
}

// ListUPS returns all UPS names and their descriptions.
func (c *Client) ListUPS() ([]UPSEntry, error) {
	if err := c.sendLine("LIST UPS"); err != nil {
		return nil, err
	}
	// expect "BEGIN LIST UPS"
	if _, err := c.readLine(); err != nil {
		return nil, fmt.Errorf("nut: LIST UPS begin: %w", err)
	}
	var result []UPSEntry
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if line == "END LIST UPS" {
			break
		}
		// UPS <name> "<desc>"
		if strings.HasPrefix(line, "UPS ") {
			entry := parseUPSLine(line)
			result = append(result, entry)
		}
	}
	return result, nil
}

// ListVar returns all variables for the given UPS.
func (c *Client) ListVar(upsName string) ([]VarEntry, error) {
	if err := c.sendLine("LIST VAR " + escapeUPSName(upsName)); err != nil {
		return nil, err
	}
	begin, err := c.readLine()
	if err != nil {
		return nil, fmt.Errorf("nut: LIST VAR begin: %w", err)
	}
	_ = begin
	var result []VarEntry
	endMarker := "END LIST VAR " + escapeUPSName(upsName)
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if line == endMarker {
			break
		}
		// VAR <ups> <varname> "<value>"
		if strings.HasPrefix(line, "VAR ") {
			entry := parseVarLine(line)
			result = append(result, entry)
		}
	}
	return result, nil
}

// GetVar returns the value of a single variable.
func (c *Client) GetVar(upsName, varName string) (string, error) {
	cmd := fmt.Sprintf("GET VAR %s %s", escapeUPSName(upsName), varName)
	if err := c.sendLine(cmd); err != nil {
		return "", err
	}
	line, err := c.readLine()
	if err != nil {
		return "", err
	}
	// VAR <ups> <varname> "<value>"
	return parseVarValue(line), nil
}

// ListCmd returns all available instant commands for the given UPS.
func (c *Client) ListCmd(upsName string) ([]string, error) {
	if err := c.sendLine("LIST CMD " + escapeUPSName(upsName)); err != nil {
		return nil, err
	}
	if _, err := c.readLine(); err != nil {
		return nil, fmt.Errorf("nut: LIST CMD begin: %w", err)
	}
	var result []string
	endMarker := "END LIST CMD " + escapeUPSName(upsName)
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if line == endMarker {
			break
		}
		// CMD <ups> <cmdname>
		if strings.HasPrefix(line, "CMD ") {
			parts := strings.SplitN(line, " ", 3)
			if len(parts) == 3 {
				result = append(result, parts[2])
			}
		}
	}
	return result, nil
}

// ListRW returns all read-write variables for the given UPS.
func (c *Client) ListRW(upsName string) ([]VarEntry, error) {
	if err := c.sendLine("LIST RW " + escapeUPSName(upsName)); err != nil {
		return nil, err
	}
	if _, err := c.readLine(); err != nil {
		return nil, fmt.Errorf("nut: LIST RW begin: %w", err)
	}
	var result []VarEntry
	endMarker := "END LIST RW " + escapeUPSName(upsName)
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if line == endMarker {
			break
		}
		// RW <ups> <varname> "<value>"
		if strings.HasPrefix(line, "RW ") {
			// same format as VAR but prefix RW
			entry := parseRWLine(line)
			result = append(result, entry)
		}
	}
	return result, nil
}

// GetVarType returns the type string of a variable.
func (c *Client) GetVarType(upsName, varName string) (string, error) {
	cmd := fmt.Sprintf("GET TYPE %s %s", escapeUPSName(upsName), varName)
	if err := c.sendLine(cmd); err != nil {
		return "", err
	}
	line, err := c.readLine()
	if err != nil {
		return "", err
	}
	// TYPE <ups> <varname> <type>
	parts := strings.SplitN(line, " ", 4)
	if len(parts) >= 4 {
		return parts[3], nil
	}
	return "", nil
}

// GetVarDesc returns the description of a variable.
func (c *Client) GetVarDesc(upsName, varName string) (string, error) {
	cmd := fmt.Sprintf("GET DESC %s %s", escapeUPSName(upsName), varName)
	if err := c.sendLine(cmd); err != nil {
		return "", err
	}
	line, err := c.readLine()
	if err != nil {
		return "", err
	}
	// DESC <ups> <varname> "<desc>"
	return parseQuotedValue(line), nil
}

// GetCmdDesc returns the description of an instant command.
func (c *Client) GetCmdDesc(upsName, cmdName string) (string, error) {
	cmd := fmt.Sprintf("GET CMDDESC %s %s", escapeUPSName(upsName), cmdName)
	if err := c.sendLine(cmd); err != nil {
		return "", err
	}
	line, err := c.readLine()
	if err != nil {
		return "", err
	}
	return parseQuotedValue(line), nil
}

// GetUPSDesc returns the description of a UPS.
func (c *Client) GetUPSDesc(upsName string) (string, error) {
	cmd := fmt.Sprintf("GET UPSDESC %s", escapeUPSName(upsName))
	if err := c.sendLine(cmd); err != nil {
		return "", err
	}
	line, err := c.readLine()
	if err != nil {
		return "", err
	}
	return parseQuotedValue(line), nil
}

// ListEnum returns all enumeration values for a variable.
func (c *Client) ListEnum(upsName, varName string) ([]string, error) {
	cmd := fmt.Sprintf("LIST ENUM %s %s", escapeUPSName(upsName), varName)
	if err := c.sendLine(cmd); err != nil {
		return nil, err
	}
	if _, err := c.readLine(); err != nil {
		return nil, fmt.Errorf("nut: LIST ENUM begin: %w", err)
	}
	var result []string
	endMarker := fmt.Sprintf("END LIST ENUM %s %s", escapeUPSName(upsName), varName)
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if line == endMarker {
			break
		}
		// ENUM <ups> <varname> "<value>"
		if strings.HasPrefix(line, "ENUM ") {
			result = append(result, parseQuotedValue(line))
		}
	}
	return result, nil
}

// ListRange returns all range constraints for a variable.
func (c *Client) ListRange(upsName, varName string) ([]RangeEntry, error) {
	cmd := fmt.Sprintf("LIST RANGE %s %s", escapeUPSName(upsName), varName)
	if err := c.sendLine(cmd); err != nil {
		return nil, err
	}
	if _, err := c.readLine(); err != nil {
		return nil, fmt.Errorf("nut: LIST RANGE begin: %w", err)
	}
	var result []RangeEntry
	endMarker := fmt.Sprintf("END LIST RANGE %s %s", escapeUPSName(upsName), varName)
	for {
		line, err := c.readLine()
		if err != nil {
			return nil, err
		}
		if line == endMarker {
			break
		}
		// RANGE <ups> <varname> "<min>" "<max>"
		if strings.HasPrefix(line, "RANGE ") {
			entry := parseRangeLine(line)
			result = append(result, entry)
		}
	}
	return result, nil
}

// InstCmd executes an instant command on a UPS.
func (c *Client) InstCmd(upsName, cmdName string) error {
	cmd := fmt.Sprintf("INSTCMD %s %s", escapeUPSName(upsName), cmdName)
	if err := c.sendLine(cmd); err != nil {
		return err
	}
	_, err := c.readLine()
	return err
}

// FSD sends the Force Shutdown command for a UPS.
func (c *Client) FSD(upsName string) error {
	cmd := fmt.Sprintf("FSD %s", escapeUPSName(upsName))
	if err := c.sendLine(cmd); err != nil {
		return err
	}
	_, err := c.readLine()
	return err
}

// SetVar sets a writable variable on a UPS.
func (c *Client) SetVar(upsName, varName, value string) error {
	cmd := fmt.Sprintf("SET VAR %s %s %q", escapeUPSName(upsName), varName, value)
	if err := c.sendLine(cmd); err != nil {
		return err
	}
	_, err := c.readLine()
	return err
}

// Login logs in to the NUT server for a given UPS (enables monitoring).
func (c *Client) Login(upsName string) error {
	if err := c.sendLine("LOGIN " + escapeUPSName(upsName)); err != nil {
		return err
	}
	_, err := c.readLine()
	return err
}

// escapeUPSName escapes a UPS name for use in NUT commands (wraps in quotes if needed).
func escapeUPSName(name string) string {
	if strings.ContainsAny(name, " \t\n") {
		return fmt.Sprintf("%q", name)
	}
	return name
}

// parseUPSLine parses a "UPS <name> "<desc>"" line.
func parseUPSLine(line string) UPSEntry {
	// format: UPS <name> "<desc>"
	parts := strings.SplitN(line, " ", 3)
	entry := UPSEntry{}
	if len(parts) >= 2 {
		entry.Name = parts[1]
	}
	if len(parts) >= 3 {
		entry.Description = unquote(parts[2])
	}
	return entry
}

// parseVarLine parses a "VAR <ups> <varname> "<value>"" line.
func parseVarLine(line string) VarEntry {
	// format: VAR <ups> <varname> "<value>"
	parts := strings.SplitN(line, " ", 4)
	entry := VarEntry{}
	if len(parts) >= 3 {
		entry.Name = parts[2]
	}
	if len(parts) >= 4 {
		entry.Value = unquote(parts[3])
	}
	return entry
}

// parseRWLine parses a "RW <ups> <varname> "<value>"" line.
func parseRWLine(line string) VarEntry {
	parts := strings.SplitN(line, " ", 4)
	entry := VarEntry{}
	if len(parts) >= 3 {
		entry.Name = parts[2]
	}
	if len(parts) >= 4 {
		entry.Value = unquote(parts[3])
	}
	return entry
}

// parseVarValue extracts the value from a "VAR <ups> <varname> "<value>"" line.
func parseVarValue(line string) string {
	parts := strings.SplitN(line, " ", 4)
	if len(parts) >= 4 {
		return unquote(parts[3])
	}
	return ""
}

// parseQuotedValue extracts the last quoted value from a NUT response line.
func parseQuotedValue(line string) string {
	idx := strings.Index(line, "\"")
	if idx < 0 {
		return ""
	}
	return unquote(line[idx:])
}

// parseRangeLine parses a "RANGE <ups> <varname> "<min>" "<max>"" line.
func parseRangeLine(line string) RangeEntry {
	parts := strings.SplitN(line, " ", 5)
	entry := RangeEntry{}
	if len(parts) >= 4 {
		entry.Min = unquote(parts[3])
	}
	if len(parts) >= 5 {
		entry.Max = unquote(parts[4])
	}
	return entry
}

// unquote removes surrounding double quotes and unescapes a NUT-quoted string.
func unquote(s string) string {
	s = strings.TrimSpace(s)
	if len(s) >= 2 && s[0] == '"' && s[len(s)-1] == '"' {
		s = s[1 : len(s)-1]
	}
	// Unescape backslash sequences
	s = strings.ReplaceAll(s, "\\\"", "\"")
	s = strings.ReplaceAll(s, "\\\\", "\\")
	return s
}
