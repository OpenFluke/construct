package construct

import (
	"encoding/json"
	"fmt"
	"net"
	"sync"
	"time"

	paragon "github.com/OpenFluke/PARAGON"
)

type Cube[T paragon.Numeric] struct {
	Name       string
	Position   []float64
	UnitName   string
	Model      *paragon.Network[T]
	ServerAddr string
	AuthPass   string
	Delimiter  string
	ClampMin   float64
	ClampMax   float64
	Debug      bool
	conn       net.Conn
}

type Construct[T paragon.Numeric] struct {
	ServerAddr string     // IP:Port of the target server
	AuthPass   string     // Authentication password for the server
	Delimiter  string     // Message delimiter for the TCP protocol
	Cubes      []*Cube[T] // Array of cubes
	ClampMin   float64
	ClampMax   float64
}

type Message map[string]interface{}

func NewCube[T paragon.Numeric](name, unitName string, pos []float64, model *paragon.Network[T], server, pass, delim string) *Cube[T] {
	return &Cube[T]{
		Name:       name,
		UnitName:   unitName,
		Position:   pos,
		Model:      model,
		ServerAddr: server,
		AuthPass:   pass,
		Delimiter:  delim,
	}
}

func (c *Construct[T]) SpawnAll() {
	for _, cube := range c.Cubes {
		if err := cube.Spawn(); err != nil {
			fmt.Println(err)
		}
	}
}

func (c *Construct[T]) DestroyAllCubes() {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		fmt.Println("[Nuke] Failed to connect:", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(c.AuthPass + c.Delimiter)); err != nil {
		fmt.Println("[Nuke] Failed to auth:", err)
		return
	}
	_, _ = readResponse(conn, c.Delimiter)

	const maxRetries = 5
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := sendJSONMessage(conn, Message{"type": "get_cube_list"}, c.Delimiter); err != nil {
			fmt.Println("[Nuke] Failed to request cube list:", err)
			return
		}
		raw, err := readResponse(conn, c.Delimiter)
		if err != nil {
			fmt.Println("[Nuke] Failed to read cube list:", err)
			return
		}

		var cubeData map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &cubeData); err != nil {
			fmt.Println("[Nuke] JSON unmarshal error:", err)
			return
		}

		cubes := toStringArray(cubeData["cubes"])
		if len(cubes) == 0 {
			fmt.Println("[Nuke] All cubes cleared.")
			break
		}

		for _, cube := range cubes {
			if err := sendJSONMessage(conn, Message{
				"type":      "despawn_cube",
				"cube_name": cube,
			}, c.Delimiter); err != nil {
				fmt.Printf("[Nuke] Failed to despawn cube %s: %v\n", cube, err)
			}
		}

		fmt.Printf("[Nuke] NUKED %d cubes (pass %d)\n", len(cubes), attempt)
		time.Sleep(500 * time.Millisecond)
	}

	fmt.Println("[Nuke] Finished.")
}

func (c *Construct[T]) UnfreezeAll() {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		fmt.Println("[UnfreezeAll] Failed to connect:", err)
		return
	}
	defer conn.Close()

	if _, err := conn.Write([]byte(c.AuthPass + c.Delimiter)); err != nil {
		fmt.Println("[UnfreezeAll] Failed to auth:", err)
		return
	}
	_, _ = readResponse(conn, c.Delimiter)

	const maxRetries = 3
	for attempt := 1; attempt <= maxRetries; attempt++ {
		if err := sendJSONMessage(conn, Message{"type": "get_cube_list"}, c.Delimiter); err != nil {
			fmt.Println("[UnfreezeAll] Failed to request cube list:", err)
			return
		}
		raw, err := readResponse(conn, c.Delimiter)
		if err != nil {
			fmt.Println("[UnfreezeAll] Failed to read cube list:", err)
			return
		}

		var cubeData map[string]interface{}
		if err := json.Unmarshal([]byte(raw), &cubeData); err != nil {
			fmt.Println("[UnfreezeAll] JSON unmarshal error:", err)
			return
		}

		cubes := toStringArray(cubeData["cubes"])
		if len(cubes) == 0 {
			fmt.Println("[UnfreezeAll] No cubes to unfreeze.")
			return
		}

		for _, cube := range cubes {
			if err := sendJSONMessage(conn, Message{
				"type":      "freeze_cube",
				"cube_name": cube,
				"freeze":    false,
			}, c.Delimiter); err != nil {
				fmt.Printf("[UnfreezeAll] Failed to unfreeze cube %s: %v\n", cube, err)
			}
		}

		fmt.Printf("[UnfreezeAll] Unfroze %d cubes (pass %d)\n", len(cubes), attempt)
		time.Sleep(200 * time.Millisecond)
	}
}

func (c *Construct[T]) GetAllCubeNames() ([]string, error) {
	conn, err := net.Dial("tcp", c.ServerAddr)
	if err != nil {
		return nil, fmt.Errorf("[GetAllCubeNames] Failed to connect: %w", err)
	}
	defer conn.Close()

	// Authenticate
	if _, err := conn.Write([]byte(c.AuthPass + c.Delimiter)); err != nil {
		return nil, fmt.Errorf("[GetAllCubeNames] Auth failed: %w", err)
	}
	_, _ = readResponse(conn, c.Delimiter)

	// Request cube list
	if err := sendJSONMessage(conn, Message{"type": "get_cube_list"}, c.Delimiter); err != nil {
		return nil, fmt.Errorf("[GetAllCubeNames] Failed to request cube list: %w", err)
	}

	raw, err := readResponse(conn, c.Delimiter)
	if err != nil {
		return nil, fmt.Errorf("[GetAllCubeNames] Failed to read response: %w", err)
	}

	var cubeData map[string]interface{}
	if err := json.Unmarshal([]byte(raw), &cubeData); err != nil {
		return nil, fmt.Errorf("[GetAllCubeNames] JSON unmarshal error: %w", err)
	}

	cubeNames := toStringArray(cubeData["cubes"])
	return cubeNames, nil
}

func (c *Construct[T]) StartPulsing(actionsPerSecond int, duration time.Duration) {
	ticker := time.NewTicker(time.Second / time.Duration(actionsPerSecond))
	defer ticker.Stop()

	end := time.Now().Add(duration)
	var wg sync.WaitGroup

	for time.Now().Before(end) {
		<-ticker.C
		wg.Add(len(c.Cubes))
		for _, cube := range c.Cubes {
			go func(cube *Cube[T]) {
				defer wg.Done()
				_ = cube.PulseWithModel() // Optional: capture and log errors
			}(cube)
		}
		wg.Wait()
	}
}
