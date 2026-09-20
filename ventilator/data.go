package ventilator

import (
	"github.com/ansel1/merry/v2"
	msgSpec "github.com/ventcon/ventcon-msgspec"
)

// calculateAirlevels takes a value and returns the intake and exhaust air levels
func calculateAirlevels(value int) (int, int, error) {
	intakeLevel := (value & 0xF0) >> 4
	exhaustLevel := value & 0x0F

	if intakeLevel < 0 || intakeLevel > 10 {
		return 0, 0, merry.Errorf("Invalid intake level: %d", intakeLevel)
	}
	if exhaustLevel < 0 || exhaustLevel > 10 {
		return 0, 0, merry.Errorf("Invalid exhaust level: %d", exhaustLevel)
	}

	return intakeLevel, exhaustLevel, nil
}

func calculateTemperature(value int) float32 {
	return float32((float64(value) / 10.0) - 40.0)
}

func updateVentilatorData(data *msgSpec.VentilatorData, fun Function, value int) error {
	switch fun {
	case CommanderAndVentilationMode:
		if value < 0 || value > 3 {
			return merry.Errorf("Invalid value for CommanderAndVentilationMode: %d", value)
		}
		data.RemoteCommander = value&0x01 != 0
		data.VentilationMode = value&0x02 != 0
	case RequestedIntakeAndExhaustAirlevels:
		il, el, err := calculateAirlevels(value)
		if err != nil {
			return merry.Prepend(err, "Failed to parse IntakeAirlevel And ExhaustAirlevel")
		}
		data.RequestedIntakeAirLevel = il
		data.RequestedExhaustAirLevel = el
	case ActualIntakeAndExhaustAirlevels:
		il, el, err := calculateAirlevels(value)
		if err != nil {
			return merry.Prepend(err, "Failed to parse Actual IntakeAirlevel And Actual ExhaustAirlevel")
		}
		data.ActualIntakeAirLevel = il
		data.ActualExhaustAirLevel = el
	case ExternalSwitchPosition:
		if value < 1 || value > 3 {
			return merry.Errorf("Invalid value for ExternalSwitchPosition: %d", value)
		}
		//specification says value would be 0-2 but in reality it is 1-3, so no adjustment needed
		data.ExternalSwitchPosition = value
	case FilterStateAndFrostRisk:
		if value < 0 || value > 7 {
			return merry.Errorf("Invalid value for FilterStateAndFrostRisk: %d", value)
		}
		data.FrostRisk = value&0x01 != 0
		data.FilterDirty = value&0x02 != 0
		data.FilterInstalled = value&0x04 == 0
	case ExhaustAirTemp:
		data.ExhaustAirTemp = calculateTemperature(value)
	case RoomAirTemp:
		data.RoomAirTemp = calculateTemperature(value)
	case ExternalAirTemp:
		data.ExternalAirTemp = calculateTemperature(value)
	case InletAirTemp:
		data.InletAirTemp = calculateTemperature(value)
	case ExhaustAirHumidity:
		if value < 0 || value > 100 {
			return merry.Errorf("Invalid value for ExhaustAirHumidity: %d", value)
		}
		data.ExhaustAirHumidity = value
	case InletAirHumidity:
		if value < 0 || value > 100 {
			return merry.Errorf("Invalid value for InletAirHumidity: %d", value)
		}
		data.InletAirHumidity = value
	case VocConcentration:
		data.VocConcentration = value * 10
	default:
		return merry.Errorf("Unknown function: %d", fun)
	}
	return nil
}

func calculateNewCommanderAndVentilationMode(remoteCommander bool, ventilationMode bool) int {
	var value = 0
	if remoteCommander {
		value |= 0x01
	}
	if ventilationMode {
		value |= 0x02
	}
	return value
}

func calculateNewIntakeAirlevelAndExhaustAirlevel(intakeAirLevel int, exhaustAirLevel int) int {
	return (intakeAirLevel << 4) | exhaustAirLevel
}
