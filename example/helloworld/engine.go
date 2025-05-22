package main

import (
	"fmt"
	"math"
	"sync"
	"time"

	"construct"

	paragon "github.com/OpenFluke/PARAGON"
)

func main() {
	addr := "localhost:14000"
	pass := "my_secure_password"
	delimiter := "<???DONE???---"

	// Create Construct instance
	c := &construct.Construct[float64]{
		ServerAddr: addr,
		AuthPass:   pass,
		Delimiter:  delimiter,
		ClampMin:   -20.0,
		ClampMax:   20.0,
	}

	goal := []float64{100, 0, 0} // Target position

	// Create cubes with simple random models
	for i := 0; i < 5; i++ {
		model := paragon.NewNetwork[float64](
			[]struct{ Width, Height int }{{3, 1}, {4, 1}, {3, 1}},
			[]string{"leakyrelu", "relu", "linear"},
			[]bool{true, true, true},
		)

		cube := &construct.Cube[float64]{
			Name:       fmt.Sprintf("cube_hello_%d", i),
			Position:   []float64{float64(i * 5), 120, 10},
			UnitName:   "HelloUnit",
			Model:      model,
			ServerAddr: addr,
			AuthPass:   pass,
			Delimiter:  delimiter,
			ClampMin:   -20.0,
			ClampMax:   20.0,
		}
		c.Cubes = append(c.Cubes, cube)
	}

	// Spawn & unfreeze
	fmt.Println("🚀 Spawning cubes...")
	c.SpawnAll()
	time.Sleep(1 * time.Second)

	fmt.Println("🧊 Unfreezing cubes...")
	c.UnfreezeAll()

	// Show active cube names
	fmt.Println("📦 Fetching cube names from server...")
	if names, err := c.GetAllCubeNames(); err != nil {
		fmt.Println("❌ Failed to fetch cube names:", err)
	} else {
		fmt.Println("✅ Cube names on server:")
		for _, name := range names {
			fmt.Println(" -", name)
		}
	}

	// Track distances during pulsing
	fmt.Println("⚡ Starting pulse + monitor loop...")

	scores := make(map[string]float64)
	var mu sync.Mutex

	// Start monitoring in parallel
	var wg sync.WaitGroup
	wg.Add(1)
	go func() {
		defer wg.Done()
		ticker := time.NewTicker(500 * time.Millisecond)
		defer ticker.Stop()
		timeout := time.After(5 * time.Second)

		for {
			select {
			case <-ticker.C:
				mu.Lock()
				for _, cube := range c.Cubes {
					dist := distance(cube.Position, goal)
					scores[cube.Name] = dist
				}
				mu.Unlock()
			case <-timeout:
				return
			}
		}
	}()

	// Run pulsing in main thread (waits for 5 seconds total)
	c.StartPulsing(100, 5*time.Second)

	// Wait for monitor thread to finish
	wg.Wait()

	// Show results
	fmt.Println("🏁 Final distances to goal:")
	var bestName string
	bestScore := math.MaxFloat64
	for name, score := range scores {
		fmt.Printf(" - %s: %.2f\n", name, score)
		if score < bestScore {
			bestScore = score
			bestName = name
		}
	}
	fmt.Printf("🥇 Best performing cube: %s (distance %.2f)\n", bestName, bestScore)

	// Cleanup
	fmt.Println("💣 Despawning cubes...")
	c.DestroyAllCubes()
}

func distance(a, b []float64) float64 {
	if len(a) != len(b) {
		return math.MaxFloat64
	}
	sum := 0.0
	for i := range a {
		diff := a[i] - b[i]
		sum += diff * diff
	}
	return math.Sqrt(sum)
}
