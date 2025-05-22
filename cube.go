package construct

import (
	"encoding/json"
	"fmt"
	"net"
)

func (c *Cube[T]) Spawn() error {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		return fmt.Errorf("❌ [%s] connect failed: %w", c.Name, err)
	}

	if _, err := conn.Write([]byte(c.AuthPass + c.Delimiter)); err != nil {
		conn.Close()
		return fmt.Errorf("❌ [%s] auth failed: %w", c.Name, err)
	}
	_, _ = readResponse(conn, c.Delimiter)

	c.conn = conn // Save the persistent connection

	cmd := Message{
		"type":      "spawn_cube",
		"cube_name": c.Name,
		"position":  c.Position,
		"rotation":  []float64{0, 0, 0},
		"is_base":   true,
	}
	if err := sendJSONMessage(c.conn, cmd, c.Delimiter); err != nil {
		c.conn.Close()
		return fmt.Errorf("❌ [%s] spawn failed: %w", c.Name, err)
	}

	c.Name += "_BASE"
	fmt.Printf("🚀 Spawned cube %s at position [%.2f, %.2f, %.2f]\n", c.Name, c.Position[0], c.Position[1], c.Position[2])
	return nil
}

func (c *Cube[T]) Despawn() error {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		return fmt.Errorf("❌ [%s] connect failed: %w", c.Name, err)
	}
	defer conn.Close()

	// Authenticate
	if _, err := conn.Write([]byte(c.AuthPass + c.Delimiter)); err != nil {
		return fmt.Errorf("❌ [%s] auth failed: %w", c.Name, err)
	}
	_, _ = readResponse(conn, c.Delimiter)

	// Send despawn command
	cmd := Message{
		"type":      "despawn_cube",
		"cube_name": c.Name,
	}
	if err := sendJSONMessage(conn, cmd, c.Delimiter); err != nil {
		return fmt.Errorf("❌ [%s] send failed: %w", c.Name, err)
	}

	fmt.Printf("💣 Despawned cube %s\n", c.Name)
	return nil
}

func (c *Cube[T]) PulseWithModel() error {
	if c.conn == nil {
		return fmt.Errorf("❌ [%s] no connection", c.Name)
	}

	input := [][]float64{
		{c.Position[0], c.Position[1], c.Position[2]},
	}
	c.Model.Forward(input)
	output := c.Model.GetOutput()

	if len(output) < 3 {
		return fmt.Errorf("❌ [%s] model output too short", c.Name)
	}

	force := make([]float64, 3)
	for i := 0; i < 3; i++ {
		v := output[i]
		if v > c.ClampMax {
			v = c.ClampMax
		}
		if v < c.ClampMin {
			v = c.ClampMin
		}
		force[i] = v
	}

	msg := Message{
		"type":  "apply_force",
		"force": force,
	}
	if err := sendJSONMessage(c.conn, msg, c.Delimiter); err != nil {
		return fmt.Errorf("❌ [%s] apply_force failed: %w", c.Name, err)
	}

	return c.RefreshPosition()
}

func (c *Cube[T]) RefreshPosition() error {
	if c.conn == nil {
		return fmt.Errorf("❌ [%s] no connection", c.Name)
	}

	request := Message{"type": "get_cube_state"}
	if err := sendJSONMessage(c.conn, request, c.Delimiter); err != nil {
		return fmt.Errorf("❌ [%s] state request failed: %w", c.Name, err)
	}

	raw, err := readResponse(c.conn, c.Delimiter)
	if err != nil {
		return fmt.Errorf("❌ [%s] state read failed: %w", c.Name, err)
	}

	var state map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &state); err != nil {
		return fmt.Errorf("❌ [%s] JSON parse error: %w", c.Name, err)
	}

	pos, ok := state["position"].([]interface{})
	if !ok || len(pos) != 3 {
		return fmt.Errorf("❌ [%s] invalid position format", c.Name)
	}

	for i := 0; i < 3; i++ {
		if val, ok := pos[i].(float64); ok {
			c.Position[i] = val
		}
	}
	return nil
}
