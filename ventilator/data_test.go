package ventilator

import (
	"fmt"
	"testing"

	"github.com/shoenig/test"
	"github.com/shoenig/test/must"
	msgspec "github.com/ventcon/ventcon-msgspec"
)

var intakeAirLevelTransformation = func(level int) int {
	switch level {
	case 0:
		return 0
	case 1:
		return 16
	case 2:
		return 32
	case 3:
		return 48
	case 4:
		return 64
	case 5:
		return 80
	case 6:
		return 96
	case 7:
		return 112
	case 8:
		return 128
	case 9:
		return 144
	case 10:
		return 160
	default:
		return -1
	}
}

func TestCalculateAirLevels(t *testing.T) {
	for intakeAirLevel := 0; intakeAirLevel <= 10; intakeAirLevel++ {
		for exhaustAirLevel := 0; exhaustAirLevel <= 10; exhaustAirLevel++ {
			t.Run(fmt.Sprintf("intakeAirLevel=%d, exhaustAirLevel=%d", intakeAirLevel, exhaustAirLevel), func(t *testing.T) {
				input := intakeAirLevelTransformation(intakeAirLevel) + exhaustAirLevel
				calculatedIntakeAirLevel, calculatedExhaustAirLevel, err := calculateAirlevels(input)
				must.NoError(t, err)
				test.Eq(t, intakeAirLevel, calculatedIntakeAirLevel)
				test.Eq(t, exhaustAirLevel, calculatedExhaustAirLevel)
			})
		}
	}
}

func TestCalculateAirLevelsErrorInvalidIntake(t *testing.T) {
	rawLevels := []int{
		-2 * 16, -1 * 16, 11 * 16, 12 * 16,
	}
	for _, rawLevel := range rawLevels {
		t.Run(fmt.Sprintf("level=%d", rawLevel), func(t *testing.T) {
			input := rawLevel + 5
			_, _, err := calculateAirlevels(input)
			test.ErrorContains(t, err, "Invalid intake level")
		})
	}
}

func TestCalculateAirLevelsErrorInvalidExhaust(t *testing.T) {
	levels := []int{
		-2, -1, 11, 12,
	}
	for _, level := range levels {
		t.Run(fmt.Sprintf("level=%d", level), func(t *testing.T) {
			input := intakeAirLevelTransformation(5) + level
			_, _, err := calculateAirlevels(input)
			test.ErrorContains(t, err, "Invalid exhaust level")
		})
	}
}

func TestCalculateNewIntakeAirlevelAndExhaustAirlevel(t *testing.T) {
	for intakeAirLevel := 0; intakeAirLevel <= 10; intakeAirLevel++ {
		for exhaustAirLevel := 0; exhaustAirLevel <= 10; exhaustAirLevel++ {
			t.Run(fmt.Sprintf("intakeAirLevel=%d, exhaustAirLevel=%d", intakeAirLevel, exhaustAirLevel), func(t *testing.T) {
				expectedOutput := intakeAirLevelTransformation(intakeAirLevel) + exhaustAirLevel
				output := calculateNewIntakeAirlevelAndExhaustAirlevel(intakeAirLevel, exhaustAirLevel)
				test.Eq(t, expectedOutput, output)
			})
		}
	}
}

func TestCalculateTemperature(t *testing.T) {
	testCases := []struct {
		code     int
		expected float32
	}{
		{614, 21.4}, // 614 -> 614/10=61,4 -> 61,4 – 40,0 = 21,4°C
		{0, -40.0},  // 0 -> 0/10=0,0 -> 0,0 – 40,0 = -40,0°C
		{400, 0.0},  // 400 -> 400/10=40,0 -> 40,0 – 40,0 = 0,0°C
		{999, 59.9}, // 1000 -> 1000/10=100,0 -> 100,0 – 40,0 = 60,0°C
	}

	for _, tc := range testCases {
		t.Run(fmt.Sprintf("temp=%.1f˚C", tc.expected), func(t *testing.T) {
			result := calculateTemperature(tc.code)
			test.Eq(t, tc.expected, result)
		})
	}
}

var commanderAndVentilationModeTestCases = []struct {
	remoteCommander bool
	ventilationMode bool
	code            int
}{
	{false, false, 0},
	{true, false, 1},
	{false, true, 2},
	{true, true, 3},
}

func TestCalculateNewCommanderAndVentilationMode(t *testing.T) {
	for _, tc := range commanderAndVentilationModeTestCases {
		t.Run(fmt.Sprintf("remoteCommander=%v, ventilationMode=%v", tc.remoteCommander, tc.ventilationMode), func(t *testing.T) {
			output := calculateNewCommanderAndVentilationMode(tc.remoteCommander, tc.ventilationMode)
			test.Eq(t, tc.code, output)
		})
	}
}

func TestUpdateVentilatorData_CommanderAndVentilationMode_Good(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := CommanderAndVentilationMode
	for _, tc := range commanderAndVentilationModeTestCases {
		t.Run(fmt.Sprintf("remoteCommander=%v, ventilationMode=%v", tc.remoteCommander, tc.ventilationMode), func(t *testing.T) {
			err := updateVentilatorData(data, fun, tc.code)
			must.NoError(t, err)
			test.Eq(t, tc.remoteCommander, data.RemoteCommander)
			test.Eq(t, tc.ventilationMode, data.VentilationMode)
		})
	}
}

func TestUpdateVentilatorData_CommanderAndVentilationMode_Bad(t *testing.T) {
	codes := []int{
		-1, 4, 5, 6,
	}

	data := &msgspec.VentilatorData{}
	fun := CommanderAndVentilationMode
	for _, code := range codes {
		t.Run(fmt.Sprintf("code=%d", code), func(t *testing.T) {
			err := updateVentilatorData(data, fun, code)
			test.ErrorContains(t, err, "Invalid value for CommanderAndVentilationMode")
		})
	}
}

func TestUpdateVentilatorData_IntakeAirlevelAndExhaustAirlevel_Good(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := RequestedIntakeAndExhaustAirlevels
	for intakeAirLevel := 0; intakeAirLevel <= 10; intakeAirLevel++ {
		for exhaustAirLevel := 0; exhaustAirLevel <= 10; exhaustAirLevel++ {
			t.Run(fmt.Sprintf("intakeAirLevel=%d, exhaustAirLevel=%d", intakeAirLevel, exhaustAirLevel), func(t *testing.T) {
				input := intakeAirLevelTransformation(intakeAirLevel) + exhaustAirLevel
				err := updateVentilatorData(data, fun, input)
				must.NoError(t, err)
				test.Eq(t, intakeAirLevel, data.RequestedIntakeAirLevel)
				test.Eq(t, exhaustAirLevel, data.RequestedExhaustAirLevel)
			})
		}
	}
}

func TestUpdateVentilatorData_IntakeAirlevelAndExhaustAirlevel_Bad(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := RequestedIntakeAndExhaustAirlevels
	codes := []int{
		16 + -2, 16 + -1, 16 + 11, 16 + 12, -2 * 16, -1 * 16, 11 * 16, 12 * 16,
	}
	for _, code := range codes {
		t.Run(fmt.Sprintf("code=%d", code), func(t *testing.T) {
			err := updateVentilatorData(data, fun, code)
			test.ErrorContains(t, err, "Failed to parse IntakeAirlevel And ExhaustAirlevel")
		})
	}
}

func TestUpdateVentilatorData_ActualIntakeAirlevelAndExhaustAirlevel_Good(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := ActualIntakeAndExhaustAirlevels
	for intakeAirLevel := 0; intakeAirLevel <= 10; intakeAirLevel++ {
		for exhaustAirLevel := 0; exhaustAirLevel <= 10; exhaustAirLevel++ {
			t.Run(fmt.Sprintf("intakeAirLevel=%d, exhaustAirLevel=%d", intakeAirLevel, exhaustAirLevel), func(t *testing.T) {
				input := intakeAirLevelTransformation(intakeAirLevel) + exhaustAirLevel
				err := updateVentilatorData(data, fun, input)
				must.NoError(t, err)
				test.Eq(t, intakeAirLevel, data.ActualIntakeAirLevel)
				test.Eq(t, exhaustAirLevel, data.ActualExhaustAirLevel)
			})
		}
	}
}

func TestUpdateVentilatorData_AcutalIntakeAirlevelAndExhaustAirlevel_Bad(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := ActualIntakeAndExhaustAirlevels
	codes := []int{
		16 + -2, 16 + -1, 16 + 11, 16 + 12, -2 * 16, -1 * 16, 11 * 16, 12 * 16,
	}
	for _, code := range codes {
		t.Run(fmt.Sprintf("code=%d", code), func(t *testing.T) {
			err := updateVentilatorData(data, fun, code)
			test.ErrorContains(t, err, "Failed to parse Actual IntakeAirlevel And Actual ExhaustAirlevel")
		})
	}
}

func TestUpdateVentilatorData_ExternalSwitchPosition_Good(t *testing.T) {
	codes := []int{
		1, 2, 3, // The manual is wrong. Instead of 0-2, it is 1-3
	}

	data := &msgspec.VentilatorData{}
	fun := ExternalSwitchPosition
	for _, code := range codes {
		t.Run(fmt.Sprintf("code=%d", code), func(t *testing.T) {
			err := updateVentilatorData(data, fun, code)
			must.NoError(t, err)
			test.Eq(t, code, data.ExternalSwitchPosition)
		})
	}
}

func TestUpdateVentilatorData_ExternalSwitchPosition_Bad(t *testing.T) {
	codes := []int{
		-1, 0, 4, 5,
	}

	data := &msgspec.VentilatorData{}
	fun := ExternalSwitchPosition
	for _, code := range codes {
		t.Run(fmt.Sprintf("code=%d", code), func(t *testing.T) {
			err := updateVentilatorData(data, fun, code)
			test.ErrorContains(t, err, "Invalid value for ExternalSwitchPosition")
		})
	}
}

func TestUpdateVentilatorData_FilterStateAndFrostRisk_Good(t *testing.T) {
	testCases := []struct {
		filterInstalled bool
		filterDirty     bool
		frostRisk       bool
		code            int
	}{
		{true, false, false, 0},
		{true, false, true, 1},
		{true, true, false, 2},
		{true, true, true, 3},
		{false, false, false, 4},
		{false, false, true, 5},
		{false, true, false, 6},
		{false, true, true, 7},
	}
	data := &msgspec.VentilatorData{}
	fun := FilterStateAndFrostRisk
	for _, tc := range testCases {
		t.Run(fmt.Sprintf("filterInstalled=%v, filterDirty=%v, frostRisk=%v", tc.filterInstalled, tc.filterDirty, tc.frostRisk), func(t *testing.T) {
			err := updateVentilatorData(data, fun, tc.code)
			must.NoError(t, err)
			test.Eq(t, tc.filterInstalled, data.FilterInstalled)
			test.Eq(t, tc.filterDirty, data.FilterDirty)
			test.Eq(t, tc.frostRisk, data.FrostRisk)
		})
	}
}

func TestUpdateVentilatorData_FilterStateAndFrostRisk_Bad(t *testing.T) {
	codes := []int{
		-2, -1, 8, 9,
	}

	data := &msgspec.VentilatorData{}
	fun := FilterStateAndFrostRisk
	for _, code := range codes {
		t.Run(fmt.Sprintf("code=%d", code), func(t *testing.T) {
			err := updateVentilatorData(data, fun, code)
			test.ErrorContains(t, err, "Invalid value for FilterStateAndFrostRisk")
		})
	}
}

func TestUpdateVentilatorData_ExhaustAirTemp(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := ExhaustAirTemp
	code := 614

	err := updateVentilatorData(data, fun, code)
	must.NoError(t, err)
	test.Eq(t, 21.4, data.ExhaustAirTemp)
}

func TestUpdateVentilatorData_RoomAirTemp(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := RoomAirTemp
	code := 614

	err := updateVentilatorData(data, fun, code)
	must.NoError(t, err)
	test.Eq(t, 21.4, data.RoomAirTemp)
}

func TestUpdateVentilatorData_ExternalAirTemp(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := ExternalAirTemp
	code := 614

	err := updateVentilatorData(data, fun, code)
	must.NoError(t, err)
	test.Eq(t, 21.4, data.ExternalAirTemp)
}

func TestUpdateVentilatorData_InletAirTemp(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := InletAirTemp
	code := 614

	err := updateVentilatorData(data, fun, code)
	must.NoError(t, err)
	test.Eq(t, 21.4, data.InletAirTemp)
}

func TestUpdateVentilatorData_ExhaustAirHumidity_Good(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := ExhaustAirHumidity
	for humidity := 0; humidity <= 100; humidity++ {
		t.Run(fmt.Sprintf("humidity=%v", humidity), func(t *testing.T) {
			err := updateVentilatorData(data, fun, humidity)
			must.NoError(t, err)
			test.Eq(t, humidity, data.ExhaustAirHumidity)
		})
	}
}

func TestUpdateVentilatorData_ExhaustAirHumidity_Bad(t *testing.T) {
	codes := []int{
		-2, -1, 101, 102,
	}

	data := &msgspec.VentilatorData{}
	fun := ExhaustAirHumidity
	for _, code := range codes {
		t.Run(fmt.Sprintf("code=%d", code), func(t *testing.T) {
			err := updateVentilatorData(data, fun, code)
			test.ErrorContains(t, err, "Invalid value for ExhaustAirHumidity")
		})
	}
}

func TestUpdateVentilatorData_InletAirHumidity_Good(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := InletAirHumidity
	for humidity := 0; humidity <= 100; humidity++ {
		t.Run(fmt.Sprintf("humidity=%v", humidity), func(t *testing.T) {
			err := updateVentilatorData(data, fun, humidity)
			must.NoError(t, err)
			test.Eq(t, humidity, data.InletAirHumidity)
		})
	}
}

func TestUpdateVentilatorData_InletAirHumidity_Bad(t *testing.T) {
	codes := []int{
		-2, -1, 101, 102,
	}

	data := &msgspec.VentilatorData{}
	fun := InletAirHumidity
	for _, code := range codes {
		t.Run(fmt.Sprintf("code=%d", code), func(t *testing.T) {
			err := updateVentilatorData(data, fun, code)
			test.ErrorContains(t, err, "Invalid value for InletAirHumidity")
		})
	}
}

func TestUpdateVentilatorData_VocConcentration(t *testing.T) {
	data := &msgspec.VentilatorData{}
	fun := VocConcentration
	// 068 -> 068 x 10 = 680 ppm
	code := 68

	err := updateVentilatorData(data, fun, code)
	must.NoError(t, err)
	test.Eq(t, 680, data.VocConcentration)
}

func TestUpdateVentilatorData_UnknownFunction(t *testing.T) {
	const TestFunction Function = 99

	data := &msgspec.VentilatorData{}
	fun := TestFunction
	code := 00

	err := updateVentilatorData(data, fun, code)
	test.ErrorContains(t, err, "Unknown function")
}
