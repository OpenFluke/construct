package construct

import (
	"bufio"
	"encoding/json"
	"net"
	"strings"
	"time"

	paragon "github.com/OpenFluke/PARAGON"
)

func toStringArray(v interface{}) []string {
	arr := []string{}
	if v == nil {
		return arr
	}
	switch vv := v.(type) {
	case []interface{}:
		for _, item := range vv {
			if str, ok := item.(string); ok {
				arr = append(arr, str)
			}
		}
	}
	return arr
}

func readResponse(conn net.Conn, delimiter string) (string, error) {
	reader := bufio.NewReader(conn)
	conn.SetReadDeadline(time.Now().Add(3 * time.Second))
	var builder strings.Builder
	for {
		line, err := reader.ReadString('-')
		if err != nil {
			break
		}
		builder.WriteString(line)
		if strings.Contains(line, delimiter) {
			break
		}
	}
	full := strings.ReplaceAll(builder.String(), delimiter, "")
	return strings.TrimSpace(full), nil
}

func sendJSONMessage(conn net.Conn, msg Message, delimiter string) error {
	data, err := json.Marshal(msg)
	if err != nil {
		return err
	}
	data = append(data, []byte(delimiter)...)
	_, err = conn.Write(data)
	return err
}

func clampAny[T paragon.Numeric](value T, min, max float64) float64 {
	v := float64(value)
	if v > max {
		return max
	}
	if v < min {
		return min
	}
	return v
}
