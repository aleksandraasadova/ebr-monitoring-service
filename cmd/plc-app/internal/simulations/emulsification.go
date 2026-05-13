package simulations

import (
	"fmt"
	"math"
	"time"

	"github.com/aleksandraasadova/ebr-monitoring-service/cmd/plc-app/internal/plc"
)

// stageDuration is the test duration per stage. Set short for testing.
const stageDuration = 120 * time.Second

// publishInterval is how often we publish sensor values.
const publishInterval = 2 * time.Second

// Simulated pot weights (grams) shared across stage functions.
var (
	wpWeight float64 = 6560 // water pot: loaded in stage 1
	opWeight float64 = 2400 // oil pot: loaded in stage 2
	mpWeight float64 = 0    // main pot: fills during transfer stages
)

// Emulsification simulates the full 18-stage emulsification process.
// Each stage lasts stageDuration. Sensor values follow physics-based curves.
func Emulsification(plcServer *plc.PLCServer) {
	fmt.Println("[PLC] Starting emulsification simulation...")

	stages := []struct {
		name     string
		simulate func(plcServer *plc.PLCServer, elapsed, total float64)
	}{
		{"water_pot_feeding", simulateIdle},
		{"oil_pot_feeding", simulateIdle},
		{"main_pot_vacuumize", simulateVacuumize},
		{"water_pot_heating", simulateWaterPotHeating},
		{"oil_pot_heating", simulateOilPotHeating},
		{"main_pot_preheating", simulateMainPotPreheating},
		{"main_pot_water_feeding", simulateWaterTransfer},
		{"main_pot_pre_blending", simulatePreBlending},
		{"main_pot_vacuum_drawing_1", simulateVacuumize},
		{"main_pot_oil_feeding", simulateOilTransfer},
		{"main_pot_vacuum_drawing_2", simulateVacuumize},
		{"emulsifying_speed_2", simulateEmulsifying},
		{"emulsifying_speed_3", simulateEmulsifying},
		{"cooling_start", simulateCoolingStart},
		{"cooling_blending", simulateCooling},
		{"additive_feeding", simulateAdditiveStage},
		{"final_blending", simulateAdditiveStage},
		{"cooling_finish", simulateCoolingFinish},
	}

	for i, stage := range stages {
		fmt.Printf("[PLC] Stage %d: %s (%.0f min)\n", i+1, stage.name, stageDuration.Minutes())
		runStage(plcServer, stage.simulate, stageDuration)
	}

	fmt.Println("[PLC] Emulsification simulation complete.")
}

func runStage(plcServer *plc.PLCServer, fn func(*plc.PLCServer, float64, float64), duration time.Duration) {
	total := duration.Seconds()
	start := time.Now()
	for {
		elapsed := time.Since(start).Seconds()
		if elapsed >= total {
			break
		}
		fn(plcServer, elapsed, total)
		time.Sleep(publishInterval)
	}
}

func publishWeights(plcServer *plc.PLCServer) {
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_weight", wpWeight+noise()*2)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_weight", opWeight+noise()*2)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_weight", mpWeight+noise()*2)
}

// simulateIdle publishes stable room-temperature readings (loading phase).
func simulateIdle(plcServer *plc.PLCServer, elapsed, total float64) {
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_temp", 22.0+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_temp", 22.0+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", 22.0+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_mixer_rpm", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", 0.0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 0)
	publishWeights(plcServer)
}

// simulateVacuumize: vacuum drops from 0 to -0.08 MPa over the stage.
func simulateVacuumize(plcServer *plc.PLCServer, elapsed, total float64) {
	progress := elapsed / total
	vacuum := -0.08 * progress
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", vacuum+noise()*0.002)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", 22.0+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 0)
	publishWeights(plcServer)
}

// simulateWaterPotHeating: WP-TEMP-01 heats exponentially from 22 to 80°C.
func simulateWaterPotHeating(plcServer *plc.PLCServer, elapsed, total float64) {
	temp := exponentialApproach(22, 80, elapsed, total*0.75)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_temp", temp+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm", 200+noise()*5)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_temp", 22+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 0)
	publishWeights(plcServer)
}

// simulateOilPotHeating: OP-TEMP-02 heats from 22 to 80°C, water pot stays at 80.
func simulateOilPotHeating(plcServer *plc.PLCServer, elapsed, total float64) {
	oilTemp := exponentialApproach(22, 80, elapsed, total*0.75)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_temp", oilTemp+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_mixer_rpm", 200+noise()*5)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_temp", 80+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_mixer_rpm", 200+noise()*5)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 0)
	publishWeights(plcServer)
}

// simulateMainPotPreheating: MP-TEMP-03 heats from 22 to 75°C.
func simulateMainPotPreheating(plcServer *plc.PLCServer, elapsed, total float64) {
	temp := exponentialApproach(22, 75, elapsed, total*0.7)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", temp+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_temp", 80+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_temp", 80+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 0)
	publishWeights(plcServer)
}

// simulateWaterTransfer: water flows from WP to MP (stage 7).
func simulateWaterTransfer(plcServer *plc.PLCServer, elapsed, total float64) {
	transferred := wpWeight * (elapsed / total)
	wpWeight = 6560 - transferred
	mpWeight = transferred
	publish(plcServer, "ebr/equipment/VEH-001/sensor/water_pot_temp", 80+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", 80+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", 0)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", -0.07+noise()*0.002)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 0)
	publishWeights(plcServer)
}

// simulatePreBlending: main pot at 80°C, homogenizer at 200 rpm.
func simulatePreBlending(plcServer *plc.PLCServer, elapsed, total float64) {
	temp := exponentialApproach(75, 80, elapsed, total*0.5)
	rpm := exponentialApproach(0, 200, elapsed, total*0.3)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", temp+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", rpm+noise()*3)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", -0.07+noise()*0.002)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 30+noise()*2)
	publishWeights(plcServer)
}

// simulateOilTransfer: oil flows from OP to MP (stage 10).
func simulateOilTransfer(plcServer *plc.PLCServer, elapsed, total float64) {
	transferred := opWeight * (elapsed / total)
	opWeight = 2400 - transferred
	mpWeight = 6560 + transferred
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", 80+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_temp", 80+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", 200+noise()*5)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", -0.08+noise()*0.002)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 30+noise()*2)
	publishWeights(plcServer)
}

// simulateEmulsifying: homogenizer ramps to 2000 rpm, temp stays at 80.
func simulateEmulsifying(plcServer *plc.PLCServer, elapsed, total float64) {
	rpm := exponentialApproach(200, 2000, elapsed, total*0.4)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", 80+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", rpm+noise()*10)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_vacuum", -0.08+noise()*0.002)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 50+noise()*2)
	publishWeights(plcServer)
}

// simulateCoolingStart: begin cooling from 80°C, reduce rpm to 200.
func simulateCoolingStart(plcServer *plc.PLCServer, elapsed, total float64) {
	temp := exponentialApproach(80, 55, elapsed, total)
	rpm := exponentialApproach(2000, 200, elapsed, total*0.5)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", temp+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", rpm+noise()*5)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 30+noise()*2)
	publishWeights(plcServer)
}

// simulateCooling: cool from ~55 to 35°C at 200 rpm.
func simulateCooling(plcServer *plc.PLCServer, elapsed, total float64) {
	temp := exponentialApproach(55, 35, elapsed, total)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", temp+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", 200+noise()*3)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 30+noise()*2)
	publishWeights(plcServer)
}

// simulateAdditiveStage: stable at 32°C, 200 rpm.
func simulateAdditiveStage(plcServer *plc.PLCServer, elapsed, total float64) {
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", 32+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", 200+noise()*3)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 30+noise()*2)
	publishWeights(plcServer)
}

// simulateCoolingFinish: cool from 32 to 25°C.
func simulateCoolingFinish(plcServer *plc.PLCServer, elapsed, total float64) {
	temp := exponentialApproach(32, 25, elapsed, total)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_temp", temp+noise())
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_homogenizer_rpm", 100+noise()*2)
	publish(plcServer, "ebr/equipment/VEH-001/sensor/main_pot_scraper_rpm", 20+noise()*2)
	publishWeights(plcServer)
}

// exponentialApproach simulates an exponential curve from `from` to `to` over `tau` seconds.
func exponentialApproach(from, to, elapsed, tau float64) float64 {
	if tau <= 0 {
		return to
	}
	return to - (to-from)*math.Exp(-elapsed/tau)
}

// noise returns small random-ish noise using a sine oscillation (deterministic enough for simulation).
var noiseCounter float64

func noise() float64 {
	noiseCounter += 0.37
	return math.Sin(noiseCounter) * 0.5
}

func publish(plcServer *plc.PLCServer, topic string, value float64) {
	payload := fmt.Sprintf("%.4f", value)
	if err := plcServer.PublishRaw(topic, []byte(payload)); err != nil {
		// non-fatal: log only
		fmt.Printf("[PLC] warn: publish %s: %v\n", topic, err)
	}
}

// SimulateOilOverheat publishes OP-TEMP-02 rising to 92°C for 2 minutes.
// Use command "4" to trigger this to test deviation monitoring and alarm creation.
func SimulateOilOverheat(plcServer *plc.PLCServer) {
	fmt.Println("[PLC] Simulating oil pot overheating: OP-TEMP-02 → 92°C")
	start := time.Now()
	duration := 2 * time.Minute
	for time.Since(start) < duration {
		elapsed := time.Since(start).Seconds()
		temp := exponentialApproach(80, 92, elapsed, 30) // fast rise
		publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_temp", temp+noise())
		time.Sleep(publishInterval)
	}
	fmt.Println("[PLC] Overheat simulation complete.")
}

// SimulateOilRecovery simulates a sensor returning to normal (80°C) after overheating.
// Use command "5" after overheating to test the recovery path.
func SimulateOilRecovery(plcServer *plc.PLCServer) {
	fmt.Println("[PLC] Simulating oil pot sensor recovery: OP-TEMP-02 → 80°C")
	start := time.Now()
	duration := 90 * time.Second
	for time.Since(start) < duration {
		elapsed := time.Since(start).Seconds()
		temp := exponentialApproach(92, 80, elapsed, 40)
		publish(plcServer, "ebr/equipment/VEH-001/sensor/oil_pot_temp", temp+noise())
		time.Sleep(publishInterval)
	}
	fmt.Println("[PLC] Sensor recovery simulation complete.")
}
